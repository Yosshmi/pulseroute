package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"pulseroute/internal/security"
	"regexp"
)

type Input struct {
	EventID string          `json:"event_id"`
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data"`
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func (v Input) Validate() error {
	if !identifier.MatchString(v.EventID) {
		return fmt.Errorf("event_id must be 1..128 identifier characters")
	}
	if !identifier.MatchString(v.Type) {
		return fmt.Errorf("type must be 1..128 identifier characters")
	}
	if len(v.Data) == 0 || !json.Valid(v.Data) {
		return fmt.Errorf("data must be valid JSON")
	}
	return nil
}
func (v Input) Canonical() ([]byte, string, error) {
	var data any
	decoder := json.NewDecoder(bytes.NewReader(v.Data))
	decoder.UseNumber()
	d := decoder.Decode(&data)
	if d != nil {
		return nil, "", d
	}
	b, e := json.Marshal(struct {
		EventID string `json:"event_id"`
		Type    string `json:"type"`
		Data    any    `json:"data"`
	}{v.EventID, v.Type, data})
	return b, security.Hash(string(b)), e
}
