package events

import (
	"encoding/json"
	"testing"
)

func TestValidationAndCanonicalization(t *testing.T) {
	a := Input{EventID: "evt_1", Type: "order.created", Data: json.RawMessage(`{"b":2,"a":1}`)}
	if e := a.Validate(); e != nil {
		t.Fatal(e)
	}
	_, h, e := a.Canonical()
	if e != nil {
		t.Fatal(e)
	}
	a.Data = json.RawMessage(`{ "a":1, "b":2 }`)
	_, h2, _ := a.Canonical()
	if h != h2 {
		t.Fatal("JSON key order must not affect idempotency")
	}
	a.EventID = ""
	if a.Validate() == nil {
		t.Fatal("missing event ID")
	}
}
