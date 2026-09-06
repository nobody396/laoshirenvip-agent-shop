package sharedstock

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSignMatchesSharedValidationContract(t *testing.T) {
	got := Sign(map[string]string{
		"race":        "",
		"num":         "1",
		"app_id":      "42",
		"shared_code": "ABC",
		"sign":        "ignored",
	}, "secret")
	const want = "e520d6b16e117b6cd0b4fe7aa608a969"
	if got != want {
		t.Fatalf("signature mismatch: got %s want %s", got, want)
	}
}

func TestConnectUsesCoreRouteAndSignedForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shared/authentication/connect" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		form := mustForm(t, r)
		assertSignedForm(t, form, "42", "secret")
		_, _ = fmt.Fprint(w, `{"code":200,"msg":"success","data":{"shopName":"Aisou","balance":"123.45"}}`)
	}))
	defer server.Close()

	result, err := NewClient(server.URL, "42", "secret").Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.ShopName != "Aisou" || result.Balance != "123.45" {
		t.Fatalf("unexpected connection: %+v", result)
	}
}

func TestClientFallsBackToLegacyOnlyAfterDefinitiveNonJSONResponse(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		if strings.HasPrefix(r.URL.Path, "/shared/") {
			_, _ = fmt.Fprint(w, `<html>plugin disabled</html>`)
			return
		}
		_, _ = fmt.Fprint(w, `{"code":"200","data":{"shopName":"Legacy","balance":9}}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "42", "secret")
	if _, err := client.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"/shared/authentication/connect",
		"/plugin/SharedStock/api/connect",
		"/plugin/SharedStock/api/connect",
	}
	if fmt.Sprint(paths) != fmt.Sprint(want) {
		t.Fatalf("unexpected route attempts: got %v want %v", paths, want)
	}
}

func TestClientAcceptsLegacyZeroSuccessCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/shared/") {
			_, _ = fmt.Fprint(w, `<html>not installed</html>`)
			return
		}
		_, _ = fmt.Fprint(w, `{"code":0,"msg":"success","data":{"shopName":"Legacy","balance":"9.00"}}`)
	}))
	defer server.Close()

	result, err := NewClient(server.URL, "42", "secret").Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.ShopName != "Legacy" || result.Balance != "9.00" {
		t.Fatalf("unexpected legacy response: %+v", result)
	}
}

func TestClientDoesNotReplayAfterJSONBusinessFailure(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = fmt.Fprint(w, `{"code":400,"msg":"insufficient balance"}`)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "42", "secret").Trade(context.Background(), TradeRequest{
		SharedCode: "ABC", Quantity: 1, RequestNo: "ORDER-1",
	})
	var responseError *ResponseError
	if !errors.As(err, &responseError) {
		t.Fatalf("expected ResponseError, got %v", err)
	}
	if requests != 1 {
		t.Fatalf("business failure must not be replayed, requests=%d", requests)
	}
}

func TestTradeTreatsDuplicateRequestAsUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"code":400,"msg":"The request ID already exists"}`)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "42", "secret").Trade(context.Background(), TradeRequest{
		SharedCode: "ABC", Quantity: 1, RequestNo: "ORDER-1",
	})
	if !errors.Is(err, ErrRequestUncertain) {
		t.Fatalf("expected uncertain result, got %v", err)
	}
}

func TestTradeSendsExplicitContactAndPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		form := mustForm(t, r)
		if got := form.Get("contact"); got != "order-test@lsrai.shop" {
			t.Fatalf("contact = %q", got)
		}
		if got := form.Get("password"); got != "Ls123456" {
			t.Fatalf("password = %q", got)
		}
		assertSignedForm(t, form, "42", "secret")
		_, _ = fmt.Fprint(w, `{"code":200,"data":{"amount":"1.00","tradeNo":"UP-1","secret":"CARD"}}`)
	}))
	defer server.Close()

	result, err := NewClient(server.URL, "42", "secret").Trade(context.Background(), TradeRequest{
		SharedCode: "ABC", Quantity: 1, RequestNo: "ORDER-1",
		Contact: "order-test@lsrai.shop", Password: "Ls123456",
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.TradeNo) != "UP-1" {
		t.Fatalf("unexpected trade: %+v", result)
	}
}

func TestTradeTreatsNonJSONResponseAsUncertainWithoutLegacyReplay(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = fmt.Fprint(w, `<html><title>页面没有找到</title></html>`)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "42", "secret").Trade(context.Background(), TradeRequest{
		SharedCode: "ABC", Quantity: 1, RequestNo: "ORDER-UNCERTAIN",
	})
	if !errors.Is(err, ErrRequestUncertain) {
		t.Fatalf("expected uncertain trade, got %v", err)
	}
	if requests != 1 {
		t.Fatalf("uncertain trade was replayed across route families: %d requests", requests)
	}
}

func TestTradeTreatsTimeoutAsUncertainWithoutReplay(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		time.Sleep(50 * time.Millisecond)
		_, _ = fmt.Fprint(w, `{"code":200,"data":{"tradeNo":"TOO-LATE"}}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "42", "secret", WithHTTPClient(&http.Client{Timeout: 10 * time.Millisecond}))
	_, err := client.Trade(context.Background(), TradeRequest{
		SharedCode: "ABC", Quantity: 1, RequestNo: "ORDER-TIMEOUT",
	})
	if !errors.Is(err, ErrRequestUncertain) {
		t.Fatalf("expected uncertain trade after timeout, got %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("timed-out trade was replayed: %d requests", got)
	}
}

func TestTradeRejectsRequestNumberLongerThanNineteenCharacters(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = fmt.Fprint(w, `{"code":200,"data":{"tradeNo":"UNEXPECTED"}}`)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "42", "secret").Trade(context.Background(), TradeRequest{
		SharedCode: "ABC", Quantity: 1, RequestNo: "12345678901234567890",
	})
	if err == nil || !strings.Contains(err.Error(), "request_no") {
		t.Fatalf("expected request_no validation error, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("invalid request_no reached upstream: %d requests", requests)
	}
}

func TestQueryUsesCamelCaseTradeNumber(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		form := mustForm(t, r)
		if got := form.Get("tradeNo"); got != "UPSTREAM-1" {
			t.Fatalf("tradeNo = %q", got)
		}
		if got := form.Get("trade_no"); got != "" {
			t.Fatalf("unexpected snake-case trade_no = %q", got)
		}
		_, _ = fmt.Fprint(w, `{"code":200,"data":{"tradeNo":"UPSTREAM-1","status":1}}`)
	}))
	defer server.Close()

	if _, err := NewClient(server.URL, "42", "secret").Query(context.Background(), "UPSTREAM-1"); err != nil {
		t.Fatal(err)
	}
}

func mustForm(t *testing.T, r *http.Request) url.Values {
	t.Helper()
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	return r.PostForm
}

func assertSignedForm(t *testing.T, form url.Values, appID, appKey string) {
	t.Helper()
	if form.Get("app_id") != appID {
		t.Fatalf("app_id mismatch: %s", form.Get("app_id"))
	}
	values := map[string]string{}
	for key := range form {
		values[key] = form.Get(key)
	}
	if got, want := form.Get("sign"), Sign(values, appKey); got != want {
		t.Fatalf("signature mismatch: got %s want %s", got, want)
	}
}
