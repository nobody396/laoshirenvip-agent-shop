package domain

import (
	"testing"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestCalculateAmounts(t *testing.T) {
	fee, total, rate, err := CalculateAmounts(money.FromDecimal(decimal.RequireFromString("130.00")), TypeOrdinary)
	if err != nil {
		t.Fatalf("CalculateAmounts() error = %v", err)
	}
	if fee.String() != "3.90" || total.String() != "130.00" || rate != 300 {
		t.Fatalf("CalculateAmounts() = fee %s total %s rate %d", fee.String(), total.String(), rate)
	}
}

func TestCalculateAmountsRejectsInvalidInput(t *testing.T) {
	if _, _, _, err := CalculateAmounts(money.FromDecimal(decimal.Zero), TypeOrdinary); err == nil {
		t.Fatal("zero amount must fail")
	}
	if _, _, _, err := CalculateAmounts(money.FromDecimal(decimal.NewFromInt(1)), "unknown"); err == nil {
		t.Fatal("unknown invoice type must fail")
	}
	if _, _, _, err := CalculateAmounts(money.FromDecimal(decimal.NewFromInt(1)), TypeSpecial); err == nil {
		t.Fatal("special invoice must fail")
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
