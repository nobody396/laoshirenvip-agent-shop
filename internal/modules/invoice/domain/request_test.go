package domain

import (
	"testing"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestCalculateAmounts(t *testing.T) {
	tests := []struct {
		name        string
		invoiceType string
		wantFee     string
		wantTotal   string
		wantRate    int
	}{
		{name: "ordinary 3 percent", invoiceType: TypeOrdinary, wantFee: "18.90", wantTotal: "648.90", wantRate: 300},
		{name: "special 6 percent", invoiceType: TypeSpecial, wantFee: "37.80", wantTotal: "667.80", wantRate: 600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fee, total, rate, err := CalculateAmounts(money.FromDecimal(decimal.RequireFromString("630.00")), tt.invoiceType)
			if err != nil {
				t.Fatalf("CalculateAmounts() error = %v", err)
			}
			if fee.String() != tt.wantFee || total.String() != tt.wantTotal || rate != tt.wantRate {
				t.Fatalf("CalculateAmounts() = fee %s total %s rate %d", fee.String(), total.String(), rate)
			}
		})
	}
}

func TestCalculateAmountsRejectsInvalidInput(t *testing.T) {
	if _, _, _, err := CalculateAmounts(money.FromDecimal(decimal.Zero), TypeOrdinary); err == nil {
		t.Fatal("zero amount must fail")
	}
	if _, _, _, err := CalculateAmounts(money.FromDecimal(decimal.NewFromInt(1)), "unknown"); err == nil {
		t.Fatal("unknown invoice type must fail")
	}
}

func TestCalculatePaymentAmountUsesFourPercentChannelSnapshot(t *testing.T) {
	fee, total, err := CalculatePaymentAmount(
		money.FromDecimal(decimal.RequireFromString("18.90")),
		money.FromDecimal(decimal.RequireFromString("4.00")),
	)
	if err != nil {
		t.Fatalf("CalculatePaymentAmount() error = %v", err)
	}
	if fee.String() != "0.76" || total.String() != "19.66" {
		t.Fatalf("CalculatePaymentAmount() = fee %s total %s", fee.String(), total.String())
	}
}
