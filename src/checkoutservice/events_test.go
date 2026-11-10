package main

import (
	"encoding/json"
	"testing"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/checkoutservice/genproto"
)

func TestNewOrderEventCountsQuantities(t *testing.T) {
	order := &pb.OrderResult{
		OrderId:            "o-1",
		ShippingTrackingId: "track-1",
		Items: []*pb.OrderItem{
			{Item: &pb.CartItem{ProductId: "A", Quantity: 2}},
			{Item: &pb.CartItem{ProductId: "B", Quantity: 3}},
		},
	}
	total := &pb.Money{CurrencyCode: "USD", Units: 42, Nanos: 500000000}

	ev := newOrderEvent(order, "user-1", total)

	if ev.ItemsCount != 5 {
		t.Fatalf("items_count = %d, want 5", ev.ItemsCount)
	}
	if ev.OrderID != "o-1" || ev.UserID != "user-1" || ev.Currency != "USD" || ev.TotalUnits != 42 {
		t.Fatalf("unexpected event: %+v", ev)
	}

	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"order_id", "user_id", "currency", "total_units", "items_count", "created_at"} {
		if _, ok := back[k]; !ok {
			t.Errorf("json field %q missing", k)
		}
	}
}

func TestNilPublisherIsNoop(t *testing.T) {
	var p *orderEventPublisher
	p.Publish(orderEvent{OrderID: "x"}) // не должно паниковать
	p.Close()
}
