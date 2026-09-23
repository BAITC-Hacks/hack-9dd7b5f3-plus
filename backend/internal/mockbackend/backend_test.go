package mockbackend

import (
	"testing"

	"hackathon/backend/internal/catalog"
)

func load(t *testing.T) *Backend {
	cat, err := catalog.Load("../../../data")
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(cat)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPrices(t *testing.T) {
	b := load(t)
	res, err := b.Execute("calc_ogpo_price", map[string]any{"region": "almaty", "vehicle_type": "car", "drivers_iin": []any{"910512300456", "930824400789"}})
	if err != nil || res["price"] != 38000.0 {
		t.Fatalf("ogpo: %v %v", res, err)
	}
	res, err = b.Execute("calc_travel_price", map[string]any{"trip_country": "Турция", "trip_start": "2026-10-10", "trip_end": "2026-10-16", "travelers_count": 2, "traveler_max_age": 42})
	if err != nil || res["price"] != 15400.0 || res["zone"] != "C" {
		t.Fatalf("travel: %v %v", res, err)
	}
	res, err = b.Execute("cancel_policy", map[string]any{"policy_number": "SQ-CASCO-204350", "cancel_reason": "sold"})
	if err != nil || res["refund_amount"] != 163800.0 {
		t.Fatalf("cancel: %v %v", res, err)
	}
	_, err = b.Execute("find_client", map[string]any{"phone": "+77010000099"})
	if err == nil || err.Code != "not_found" {
		t.Fatalf("expected not_found, got %v", err)
	}
	res, err = b.Execute("find_client", map[string]any{"phone": "8 701 000 00 07"})
	if err != nil || res["client_id"] != "C007" {
		t.Fatalf("find: %v %v", res, err)
	}
	res, err = b.Execute("get_policy", map[string]any{"vehicle_plate": "777ABC02"})
	if err != nil || res["policy_number"] != "SQ-OGPO-104501" {
		t.Fatalf("plate: %v %v", res, err)
	}
	res, err = b.Execute("check_coverage", map[string]any{"policy_number": "SQ-DMS-604220", "service_name": "УЗИ"})
	if err != nil || res["covered"] != true {
		t.Fatalf("coverage: %v %v", res, err)
	}
}
