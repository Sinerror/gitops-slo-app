// order-consumer читает события заказов из Kafka (топик orders) и «обрабатывает» их:
// логирует и считает метрики. PROCESSING_DELAY_MS искусственно замедляет обработку —
// так воспроизводится сценарий «отставание consumer» (алерт по kafka_consumergroup_lag).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
)

type OrderEvent struct {
	OrderID    string    `json:"order_id"`
	UserID     string    `json:"user_id"`
	Currency   string    `json:"currency"`
	TotalUnits int64     `json:"total_units"`
	TotalNanos int32     `json:"total_nanos"`
	ItemsCount int       `json:"items_count"`
	CreatedAt  time.Time `json:"created_at"`
}

var (
	processed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "orders_processed_total",
		Help: "Обработанные события заказов по результату.",
	}, []string{"result"})
	processingSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "order_processing_duration_seconds",
		Help:    "Время обработки одного события.",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 14),
	})
	endToEndSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "order_event_age_seconds",
		Help:    "Возраст события в момент обработки (от оформления заказа).",
		Buckets: prometheus.ExponentialBuckets(0.01, 2, 16),
	})
	revenue = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "orders_revenue_units_total",
		Help: "Сумма заказов в целых единицах валюты.",
	}, []string{"currency"})
)

type config struct {
	brokers []string
	topic   string
	group   string
	delay   time.Duration
	addr    string
}

func loadConfig() (config, error) {
	c := config{
		brokers: strings.Split(getenv("KAFKA_BROKERS", "localhost:9092"), ","),
		topic:   getenv("KAFKA_TOPIC", "orders"),
		group:   getenv("KAFKA_GROUP", "order-consumer"),
		addr:    getenv("METRICS_ADDR", ":9102"),
	}
	ms, err := strconv.Atoi(getenv("PROCESSING_DELAY_MS", "0"))
	if err != nil || ms < 0 {
		return c, errors.New("PROCESSING_DELAY_MS must be a non-negative integer")
	}
	c.delay = time.Duration(ms) * time.Millisecond
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// handle обрабатывает одно сообщение. Битые сообщения не роняют consumer:
// они считаются в метрике и пропускаются (иначе одна ошибка блокирует партицию).
func handle(msg []byte, delay time.Duration, now func() time.Time) (OrderEvent, error) {
	var ev OrderEvent
	if err := json.Unmarshal(msg, &ev); err != nil {
		return ev, err
	}
	if ev.OrderID == "" {
		return ev, errors.New("order_id is empty")
	}
	if delay > 0 {
		time.Sleep(delay)
	}
	if !ev.CreatedAt.IsZero() {
		endToEndSeconds.Observe(now().Sub(ev.CreatedAt).Seconds())
	}
	revenue.WithLabelValues(ev.Currency).Add(float64(ev.TotalUnits))
	return ev, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := loadConfig()
	if err != nil {
		logger.Error("bad config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var lastFetchOK atomic.Int64
	lastFetchOK.Store(time.Now().Unix())

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	srv := &http.Server{Addr: cfg.addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics server", "err", err)
			stop()
		}
	}()

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.brokers,
		Topic:          cfg.topic,
		GroupID:        cfg.group,
		MinBytes:       1,
		MaxBytes:       1 << 20,
		MaxWait:        500 * time.Millisecond,
		CommitInterval: time.Second,
		StartOffset:    kafka.FirstOffset,
	})
	defer reader.Close()

	logger.Info("order-consumer started", "brokers", cfg.brokers, "topic", cfg.topic, "group", cfg.group, "delay", cfg.delay.String())

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			logger.Warn("fetch failed", "err", err)
			time.Sleep(time.Second)
			continue
		}
		lastFetchOK.Store(time.Now().Unix())

		start := time.Now()
		ev, err := handle(msg.Value, cfg.delay, time.Now)
		processingSeconds.Observe(time.Since(start).Seconds())
		if err != nil {
			processed.WithLabelValues("invalid").Inc()
			logger.Warn("invalid order event", "err", err, "partition", msg.Partition, "offset", msg.Offset)
		} else {
			processed.WithLabelValues("ok").Inc()
			logger.Info("order processed", "order_id", ev.OrderID, "items", ev.ItemsCount, "currency", ev.Currency, "partition", msg.Partition, "offset", msg.Offset)
		}
		if err := reader.CommitMessages(ctx, msg); err != nil && ctx.Err() == nil {
			logger.Warn("commit failed", "err", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	logger.Info("order-consumer stopped")
}
