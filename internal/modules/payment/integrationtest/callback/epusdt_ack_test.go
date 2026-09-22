package paymentcallback_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/dujiao-next/internal/constants"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	epusdtadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/epusdt"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epusdt"
	paymentprovider "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/provider"
	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
	paymentcallback "github.com/dujiao-next/internal/modules/payment/transport/http/callback"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"gorm.io/gorm"
)

type ackNotificationSpy struct{ calls int }

func (s *ackNotificationSpy) Enqueue(notificationcontract.EnqueueInput) error { s.calls++; return nil }

func newEpusdtACKFixture(t *testing.T) (*okpayCallbackFixture, *ackNotificationSpy) {
	t.Helper()
	f := newOkpayCallbackFixture(t)
	f.channel.ProviderType = " EPUSDT "
	f.channel.ConfigJSON = jsonmap.JSON{"gateway_url": "https://example.com", "pid": "merchant-test", "secret_key": "test-only-key", "currency": "USDT"}
	if err := f.db.Save(f.channel).Error; err != nil {
		t.Fatal(err)
	}
	f.payment.ProviderType = constants.PaymentProviderEpusdt
	if err := f.db.Save(f.payment).Error; err != nil {
		t.Fatal(err)
	}
	registry := paymentprovider.NewRegistry()
	registry.Register(constants.PaymentProviderEpusdt, "", epusdtadapter.NewEpusdtAdapter())
	spy := &ackNotificationSpy{}
	channels := paymentgormstore.NewChannelStore(f.db)
	f.service = paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{OrderStore: f.orderRepo, PaymentStore: f.paymentRepo, ChannelStore: channels, ProductRepo: productgormstore.NewProductStore(f.db), ProductSKURepo: productgormstore.NewSKUStore(f.db), PaymentProviderRegistry: registry, NotificationService: spy})
	f.handler = paymentcallback.NewHandler(callbackServiceTestAdapter{payments: f.service}, f.paymentRepo, channels, nil)
	return f, spy
}

func signedEpusdtACKBody(t *testing.T, status int, orderNo, tradeID string, amount float64, badSignature bool) string {
	t.Helper()
	p := map[string]interface{}{"pid": "merchant-test", "trade_id": tradeID, "order_id": orderNo, "amount": amount, "actual_amount": 1.0, "receive_address": "test-address", "token": "usdt", "block_transaction_id": "test-transaction", "status": status}
	p["signature"] = epusdt.Sign(p, "test-only-key")
	if badSignature {
		p["signature"] = "invalid"
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEpusdtNonpaidACKDoesNotWriteBusinessState(t *testing.T) {
	for _, status := range []int{1, 3} {
		for _, initial := range []string{constants.PaymentStatusPending, constants.PaymentStatusSuccess, constants.PaymentStatusExpired} {
			t.Run(fmt.Sprintf("status_%d_initial_%s", status, initial), func(t *testing.T) {
				f, spy := newEpusdtACKFixture(t)
				if err := f.db.Model(f.payment).Update("status", initial).Error; err != nil {
					t.Fatal(err)
				}
				// Production affected payments are expired wallet recharges (no order).
				if initial == constants.PaymentStatusExpired {
					if err := f.db.Model(f.payment).Update("order_id", 0).Error; err != nil {
						t.Fatal(err)
					}
				}
				writes := 0
				before := func(*gorm.DB) { writes++ }
				f.db.Callback().Create().Before("gorm:create").Register("ack_assert_create", before)
				f.db.Callback().Update().Before("gorm:update").Register("ack_assert_update", before)
				f.db.Callback().Delete().Before("gorm:delete").Register("ack_assert_delete", before)
				f.db.Callback().Raw().Before("gorm:raw").Register("ack_assert_raw_write", before)
				body := signedEpusdtACKBody(t, status, f.payment.GatewayOrderNo, f.payment.ProviderRef, 616, false)
				for i := 0; i < 2; i++ { // natural retries stay no-op, including after an earlier paid event
					w := performOkpayJSONCallback(t, f, body)
					if w.Code != http.StatusOK || w.Body.String() != "ok" {
						t.Fatalf("ACK = %d %q", w.Code, w.Body.String())
					}
				}
				if writes != 0 || spy.calls != 0 {
					t.Fatalf("nonpaid callback caused %d DB writes, %d notification enqueues", writes, spy.calls)
				}
				payment, err := f.paymentRepo.GetByID(f.payment.ID)
				if err != nil {
					t.Fatal(err)
				}
				order, err := f.orderRepo.GetByID(f.order.ID)
				if err != nil {
					t.Fatal(err)
				}
				if payment.Status != initial || order.Status != constants.OrderStatusPendingPayment || order.PaidAt != nil {
					t.Fatal("business state changed")
				}
			})
		}
	}
}

func TestEpusdtACKRejectsInvalidSignatureStatusAndOwnership(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		status                               int
		wrongOrder, wrongTrade, badSignature bool
	}{
		{"bad-signature-paid", 2, false, false, true},
		{"bad-signature-waiting", 1, false, false, true}, {"bad-signature-expired", 3, false, false, true},
		{"unknown-status", 9, false, false, false}, {"wrong-trade", 1, false, true, false}, {"wrong-order", 3, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, spy := newEpusdtACKFixture(t)
			orderNo, tradeID := f.payment.GatewayOrderNo, f.payment.ProviderRef
			if tc.wrongOrder {
				orderNo = "unrelated-order"
			}
			if tc.wrongTrade {
				tradeID = "unrelated-trade"
			}
			writes := 0
			f.db.Callback().Update().Before("gorm:update").Register("reject_assert_update", func(*gorm.DB) { writes++ })
			w := performOkpayJSONCallback(t, f, signedEpusdtACKBody(t, tc.status, orderNo, tradeID, 616, tc.badSignature))
			if w.Code != http.StatusOK || w.Body.String() != "fail" {
				t.Fatalf("invalid ACK = %d %q", w.Code, w.Body.String())
			}
			if writes != 0 || spy.calls != 0 {
				t.Fatal("invalid callback mutated state or notified")
			}
		})
	}
	t.Run("wrong-channel", func(t *testing.T) {
		f, spy := newEpusdtACKFixture(t)
		wrong := *f.channel
		wrong.ID += 100
		_, err := f.service.HandleSyncCallback(&wrong, nil, []byte(signedEpusdtACKBody(t, 1, f.payment.GatewayOrderNo, f.payment.ProviderRef, 616, false)))
		if err == nil || spy.calls != 0 {
			t.Fatal("cross-channel callback accepted")
		}
	})
}

func TestEpusdtPaidKeepsAmountValidationAndPaidTransition(t *testing.T) {
	for _, amount := range []float64{615, 616} {
		t.Run(fmt.Sprintf("amount_%v", amount), func(t *testing.T) {
			f, _ := newEpusdtACKFixture(t)
			w := performOkpayJSONCallback(t, f, signedEpusdtACKBody(t, 2, f.payment.GatewayOrderNo, f.payment.ProviderRef, amount, false))
			want := "fail"
			if amount == 616 {
				want = "ok"
			}
			if w.Code != http.StatusOK || w.Body.String() != want {
				t.Fatalf("paid ACK = %d %q want %s", w.Code, w.Body.String(), want)
			}
			p, err := f.paymentRepo.GetByID(f.payment.ID)
			if err != nil {
				t.Fatal(err)
			}
			expected := constants.PaymentStatusPending
			if amount == 616 {
				expected = constants.PaymentStatusSuccess
			}
			if p.Status != expected {
				t.Fatalf("payment status %s want %s", p.Status, expected)
			}
		})
	}
}
