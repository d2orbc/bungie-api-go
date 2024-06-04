package bnet

import (
	"encoding/json"
	"fmt"
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

func TestNullable(t *testing.T) {
	var v float64 = 1.33333333333
	var a Nullable[float64] = Nullable[float64]{v: &v}

	if got, want := fmt.Sprintf("%0.2f", a), "1.33"; got != want {
		t.Fatalf("want %s; got %s", want, got)
	}

	a = Nullable[float64]{v: nil}

	if got, want := fmt.Sprintf("%0.2f", a), "null"; got != want {
		t.Fatalf("want %s; got %s", want, got)
	}
}
