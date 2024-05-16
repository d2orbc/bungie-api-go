package bnet

import (
	"encoding/json"
	"testing"
)

func TestInt64(t *testing.T) {
	inJSON := `{"a":"100"}`
	var s struct {
		A Int64
	}
	if err := json.Unmarshal([]byte(inJSON), &s); err != nil {
		t.Fatal(err)
	}
	if s.A != 100 {
		t.Fatalf("s=%v", s)
	}
}
