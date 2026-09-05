package integrationtrace

import (
	"reflect"
	"testing"
)

func TestSanitizeJSONRedactsCredentialsAndFulfillment(t *testing.T) {
	value := SanitizeJSON([]byte(`{"code":200,"data":{"trade_no":"T-1","secret":"CARD-1","nested":{"sign":"abc"}}}`))
	want := map[string]any{
		"code": float64(200),
		"data": map[string]any{
			"trade_no": "T-1",
			"secret":   redacted,
			"nested":   map[string]any{"sign": redacted},
		},
	}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("SanitizeJSON() = %#v, want %#v", value, want)
	}
}

func TestURLSummaryRedactsSignature(t *testing.T) {
	got := URLSummary("https://pay.example.com/submit.php?out_trade_no=DJP-1&sign=secret")
	query, ok := got["query"].(map[string][]string)
	if !ok {
		t.Fatalf("query type = %T", got["query"])
	}
	if query["sign"][0] != redacted || query["out_trade_no"][0] != "DJP-1" {
		t.Fatalf("unexpected sanitized query: %#v", query)
	}
}
