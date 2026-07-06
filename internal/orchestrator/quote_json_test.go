package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQuoteResultBuildsJSON(t *testing.T) {
	r := &QuoteResult{
		Builds: map[string]*BuildResult{
			VariantSelfFunded:    &BuildResult{Transaction: "aaa", Variant: VariantSelfFunded, TransactionSizeBytes: 100},
			VariantSelfFundedMev: &BuildResult{Transaction: "bbb", Variant: VariantSelfFundedMev, TransactionSizeBytes: 150, JitoTipLamports: 1000},
		},
		Capabilities: map[string]Capability{
			VariantSelfFunded:    {Supported: true},
			VariantSelfFundedMev: {Supported: true},
		},
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"builds"`) {
		t.Fatalf("missing builds key: %s", s)
	}
	if !strings.Contains(s, `"selfFundedMev"`) {
		t.Fatalf("missing selfFundedMev key: %s", s)
	}
	if !strings.Contains(s, `"bbb"`) {
		t.Fatalf("missing selfFundedMev transaction: %s", s)
	}
}
