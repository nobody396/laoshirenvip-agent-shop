package epay

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQueryPaymentV2VerifiesResponseAndReturnsPaidOrder(t *testing.T) {
	merchantPrivate, _ := generateEpayRSAKeyPair(t)
	platformPrivate, platformPublic := generateEpayRSAKeyPair(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != epayQueryPathV2 || r.Method != http.MethodPost {
			t.Fatalf("query request = %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse query form: %v", err)
		}
		if r.Form.Get("pid") != "1001" || r.Form.Get("trade_no") != "EPAY-PAID-1" || r.Form.Get("timestamp") == "" {
			t.Fatalf("unexpected query form: %v", r.Form)
		}
		fields := map[string]string{
			"code": "0", "msg": "success", "trade_no": "EPAY-PAID-1",
			"out_trade_no": "DJP-1", "type": "alipay", "status": "1",
			"money": "150.00", "timestamp": "1721206072",
		}
		sign, err := signRSA(buildSignContent(fields), platformPrivate)
		if err != nil {
			t.Fatalf("sign query response: %v", err)
		}
		payload := map[string]interface{}{}
		for key, value := range fields {
			payload[key] = value
		}
		payload["code"] = 0
		payload["status"] = 1
		payload["sign"] = sign
		payload["sign_type"] = epaySignTypeRSA
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer server.Close()

	cfg := &Config{
		GatewayURL: server.URL, EpayVersion: VersionV2, MerchantID: "1001",
		PrivateKey: merchantPrivate, PublicKey: platformPublic,
	}
	cfg.Normalize()
	result, err := QueryPayment(context.Background(), cfg, "EPAY-PAID-1")
	if err != nil {
		t.Fatalf("query payment: %v", err)
	}
	if result.Status != 1 || result.TradeNo != "EPAY-PAID-1" || result.OutTradeNo != "DJP-1" || result.Money != "150.00" {
		t.Fatalf("unexpected query result: %+v", result)
	}
}

func TestQueryPaymentV2RejectsUnsignedResponse(t *testing.T) {
	merchantPrivate, _ := generateEpayRSAKeyPair(t)
	_, platformPublic := generateEpayRSAKeyPair(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"trade_no":"EPAY-1","out_trade_no":"DJP-1","status":1,"money":"1.00","timestamp":"1721206072"}`))
	}))
	defer server.Close()
	cfg := &Config{GatewayURL: server.URL, EpayVersion: VersionV2, MerchantID: "1001", PrivateKey: merchantPrivate, PublicKey: platformPublic}
	cfg.Normalize()
	if _, err := QueryPayment(context.Background(), cfg, "EPAY-1"); err != ErrSignatureInvalid {
		t.Fatalf("unsigned query error = %v, want ErrSignatureInvalid", err)
	}
}
