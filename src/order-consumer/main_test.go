package main

import (
	"testing"
	"time"
)

func TestHandleValidEvent(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 1, 0, time.UTC)
	msg := []byte(`{"order_id":"o-1","user_id":"u","currency":"USD","total_units":10,"items_count":2,"created_at":"2026-01-01T12:00:00Z"}`)

	ev, err := handle(msg, 0, func() time.Time { return now })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.OrderID != "o-1" || ev.ItemsCount != 2 || ev.TotalUnits != 10 {
		t.Fatalf("unexpected event: %+v", ev)
	}
}

func TestHandleRejectsGarbage(t *testing.T) {
	cases := map[string][]byte{
		"not json":       []byte("not json"),
		"empty order id": []byte(`{"user_id":"u"}`),
	}
	for name, msg := range cases {
		if _, err := handle(msg, 0, time.Now); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestHandleAppliesDelay(t *testing.T) {
	start := time.Now()
	if _, err := handle([]byte(`{"order_id":"o"}`), 30*time.Millisecond, time.Now); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < 30*time.Millisecond {
		t.Fatal("delay was not applied")
	}
}

func TestLoadConfigValidatesDelay(t *testing.T) {
	t.Setenv("PROCESSING_DELAY_MS", "-5")
	if _, err := loadConfig(); err == nil {
		t.Fatal("expected error for negative delay")
	}
	t.Setenv("PROCESSING_DELAY_MS", "250")
	c, err := loadConfig()
	if err != nil || c.delay != 250*time.Millisecond {
		t.Fatalf("got %v, %v", c.delay, err)
	}
}
