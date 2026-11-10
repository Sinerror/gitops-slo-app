// Публикация событий заказов в Kafka — добавлено поверх апстрима Online Boutique.
//
// Отправка асинхронная: оформление заказа не ждёт Kafka и не падает, если брокер
// недоступен (событие логируется как потерянное). Для учебного стенда это
// осознанный компромисс; в проде здесь был бы transactional outbox.
package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/checkoutservice/genproto"
	"github.com/segmentio/kafka-go"
)

type orderEvent struct {
	OrderID    string    `json:"order_id"`
	UserID     string    `json:"user_id"`
	Currency   string    `json:"currency"`
	TotalUnits int64     `json:"total_units"`
	TotalNanos int32     `json:"total_nanos"`
	ItemsCount int       `json:"items_count"`
	TrackingID string    `json:"shipping_tracking_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func newOrderEvent(o *pb.OrderResult, userID string, total *pb.Money) orderEvent {
	items := 0
	for _, it := range o.GetItems() {
		items += int(it.GetItem().GetQuantity())
	}
	return orderEvent{
		OrderID:    o.GetOrderId(),
		UserID:     userID,
		Currency:   total.GetCurrencyCode(),
		TotalUnits: total.GetUnits(),
		TotalNanos: total.GetNanos(),
		ItemsCount: items,
		TrackingID: o.GetShippingTrackingId(),
		CreatedAt:  time.Now().UTC(),
	}
}

type orderEventPublisher struct {
	writer *kafka.Writer
}

// newOrderEventPublisherFromEnv возвращает nil, если KAFKA_BROKERS не задан —
// тогда сервис ведёт себя как апстрим.
func newOrderEventPublisherFromEnv() *orderEventPublisher {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		log.Info("KAFKA_BROKERS not set, order events disabled")
		return nil
	}
	topic := os.Getenv("KAFKA_TOPIC")
	if topic == "" {
		topic = "orders"
	}
	log.Infof("order events enabled: brokers=%s topic=%s", brokers, topic)
	return &orderEventPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(strings.Split(brokers, ",")...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
			Async:        true,
			BatchTimeout: 50 * time.Millisecond,
			WriteTimeout: 5 * time.Second,
			Completion: func(msgs []kafka.Message, err error) {
				if err != nil {
					log.Warnf("failed to publish %d order event(s): %v", len(msgs), err)
				}
			},
		},
	}
}

// Publish безопасен для nil-получателя (события выключены).
func (p *orderEventPublisher) Publish(ev orderEvent) {
	if p == nil {
		return
	}
	body, err := json.Marshal(ev)
	if err != nil {
		log.Warnf("failed to marshal order event: %v", err)
		return
	}
	// Async writer: WriteMessages только ставит сообщение в очередь
	if err := p.writer.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(ev.UserID),
		Value: body,
	}); err != nil {
		log.Warnf("failed to enqueue order event %s: %v", ev.OrderID, err)
	}
}

func (p *orderEventPublisher) Close() {
	if p == nil {
		return
	}
	_ = p.writer.Close()
}
