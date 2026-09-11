package invoicehttp

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestInvoiceOrderAmountIncludesCustomerPaidChannelFee(t *testing.T) {
	order := &orderdomain.Order{
		WalletPaidAmount: money.FromDecimal(decimal.NewFromInt(3)),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(747)),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(750)),
	}
	payments := []paymentdomain.Payment{{
		ProviderType: "epay", Status: constants.PaymentStatusSuccess,
		Amount:    money.FromDecimal(decimal.RequireFromString("762.50")),
		FeeAmount: money.FromDecimal(decimal.RequireFromString("15.50")), FeePolicy: constants.PaymentFeePolicyCustomerSurcharge,
	}}
	if got := invoiceOrderAmount(order, payments); got.String() != "765.50" {
		t.Fatalf("invoiceOrderAmount() = %s, want wallet 3.00 + online charge 762.50", got.String())
	}
}

func TestInvoiceOrderAmountUsesWalletPaymentOnce(t *testing.T) {
	order := &orderdomain.Order{WalletPaidAmount: money.FromDecimal(decimal.NewFromInt(750)), TotalAmount: money.FromDecimal(decimal.NewFromInt(750))}
	payments := []paymentdomain.Payment{{ProviderType: constants.PaymentProviderWallet, Status: constants.PaymentStatusSuccess, Amount: money.FromDecimal(decimal.NewFromInt(750))}}
	if got := invoiceOrderAmount(order, payments); got.String() != "750.00" {
		t.Fatalf("invoiceOrderAmount() = %s, want 750.00", got.String())
	}
}

func TestParseInvoiceAmountUsesDeclaredFaceAmount(t *testing.T) {
	amount, err := parseInvoiceAmount("130.00", money.FromDecimal(decimal.RequireFromString("117.00")))
	if err != nil {
		t.Fatal(err)
	}
	if amount.String() != "130.00" {
		t.Fatalf("parseInvoiceAmount() = %s, want 130.00", amount.String())
	}
}
