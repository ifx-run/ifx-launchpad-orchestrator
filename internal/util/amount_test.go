package util

import "testing"

func TestParseUIAmount(t *testing.T) {
	raw, err := ParseUIAmount("1.5", 9)
	if err != nil {
		t.Fatal(err)
	}
	if raw != 1_500_000_000 {
		t.Fatalf("got %d", raw)
	}
}

func TestResolveInputAmount_prefersRaw(t *testing.T) {
	raw, err := ResolveInputAmount("1.0", "1000123456", 6)
	if err != nil {
		t.Fatal(err)
	}
	if raw != 1_000_123_456 {
		t.Fatalf("got %d", raw)
	}
}

func TestResolveInputAmount_fallsBackToUI(t *testing.T) {
	raw, err := ResolveInputAmount("1.5", "", 9)
	if err != nil {
		t.Fatal(err)
	}
	if raw != 1_500_000_000 {
		t.Fatalf("got %d", raw)
	}
}
