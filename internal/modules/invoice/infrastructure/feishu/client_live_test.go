package feishu

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/invoice/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestLiveUpsertPaidRequest(t *testing.T) {
	secret := os.Getenv("LIVE_FEISHU_APP_SECRET")
	if secret == "" { t.Skip("LIVE_FEISHU_APP_SECRET is not set") }
	client := New(Config{
		AppID: "cli_a9464c9467395cdd", AppSecret: secret,
		BaseToken: "SuYZbS5eqaPbT2st6jZcdMp5n7d", TableID: "tblpFHvA1pWIAmOE",
	})
	now := time.Now()
	request := &domain.Request{
		RequestNo: "INV-LIVE-" + now.Format("20060102150405"), Source: "dujiao", SourceHost: "lsrai.shop",
		OriginalOrderNo: "TEST-NO-MONEY", InvoiceType: domain.TypeOrdinary, RateBPS: 300,
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(1)), InvoiceFeeAmount: money.FromDecimal(decimal.RequireFromString("0.03")),
		InvoiceTotalAmount: money.FromDecimal(decimal.RequireFromString("1.03")), PaymentFeeRate: money.FromDecimal(decimal.NewFromInt(4)),
		PaymentFeeAmount: money.FromDecimal(decimal.Zero), PaymentAmount: money.FromDecimal(decimal.RequireFromString("0.03")),
		BuyerTitle: "开票链路验收测试", TaxNumber: "TEST00000000000000", RecipientEmail: "test@example.com", ProviderRef: "TEST-PROVIDER", CreatedAt: now,
	}
	first, err := client.UpsertPaidRequest(context.Background(), request)
	if err != nil { t.Fatalf("UpsertPaidRequest() error = %v", err) }
	second, err := client.UpsertPaidRequest(context.Background(), request)
	if err != nil { t.Fatalf("second UpsertPaidRequest() error = %v", err) }
	if first == "" || first != second { t.Fatalf("upsert record IDs differ: %q %q", first, second) }
	t.Logf("record_id=%s", first)
}
