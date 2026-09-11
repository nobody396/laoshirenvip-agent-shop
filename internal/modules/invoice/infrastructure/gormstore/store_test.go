package gormstore

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/invoice/domain"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func openWalletInvoiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:invoice_wallet_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&domain.Request{}, &walletdomain.Account{}, &walletdomain.Transaction{}, &walletdomain.ResellerAccount{}, &walletdomain.ResellerTransaction{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func walletInvoiceRequest(orderNo string) *domain.Request {
	return &domain.Request{
		RequestNo: "INV-" + orderNo, Source: "dujiao", SourceHost: "lsrai.shop", OriginalOrderNo: orderNo,
		InvoiceType: domain.TypeOrdinary, OriginalAmount: money.FromDecimal(decimal.NewFromInt(117)),
		InvoiceFeeAmount: money.FromDecimal(decimal.RequireFromString("3.51")), InvoiceTotalAmount: money.FromDecimal(decimal.RequireFromString("120.51")),
		PaymentAmount: money.FromDecimal(decimal.RequireFromString("3.51")), BuyerTitle: "示例公司", TaxNumber: "TEST", RecipientEmail: "test@example.com",
		Status: domain.StatusPendingPayment,
	}
}

func TestCreateAndPayWithMainWalletIsAtomic(t *testing.T) {
	db := openWalletInvoiceDB(t)
	account := walletdomain.Account{UserID: 7, Balance: money.FromDecimal(decimal.NewFromInt(10))}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	request := walletInvoiceRequest("MAIN-1")
	if err := New(db).CreateAndPayWithWallet(request, 7, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&account, account.ID).Error; err != nil {
		t.Fatal(err)
	}
	if request.Status != domain.StatusPendingIssue || request.PaidAt == nil || account.Balance.String() != "6.49" {
		t.Fatalf("unexpected request/account: request=%+v balance=%s", request, account.Balance.String())
	}
	var entry walletdomain.Transaction
	if err := db.Where("reference = ?", "invoice:"+request.RequestNo+":wallet").First(&entry).Error; err != nil {
		t.Fatal(err)
	}
	if entry.Amount.String() != "3.51" || entry.BalanceAfter.String() != "6.49" || entry.OrderID != nil {
		t.Fatalf("unexpected wallet entry: %+v", entry)
	}
}

func TestCreateAndPayWithResellerWalletUsesTenantBalance(t *testing.T) {
	db := openWalletInvoiceDB(t)
	account := walletdomain.ResellerAccount{ResellerID: 18, UserID: 7, Balance: money.FromDecimal(decimal.NewFromInt(5))}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	request := walletInvoiceRequest("RESELLER-1")
	resellerID := uint(18)
	if err := New(db).CreateAndPayWithWallet(request, 7, &resellerID); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&account, account.ID).Error; err != nil {
		t.Fatal(err)
	}
	if account.Balance.String() != "1.49" {
		t.Fatalf("unexpected reseller balance: %s", account.Balance.String())
	}
	var mainCount, resellerCount int64
	_ = db.Model(&walletdomain.Transaction{}).Count(&mainCount).Error
	_ = db.Model(&walletdomain.ResellerTransaction{}).Where("reseller_id = ?", resellerID).Count(&resellerCount).Error
	if mainCount != 0 || resellerCount != 1 {
		t.Fatalf("wallet scope crossed: main=%d reseller=%d", mainCount, resellerCount)
	}
}

func TestCreateAndPayWithWalletRollsBackWhenBalanceIsInsufficient(t *testing.T) {
	db := openWalletInvoiceDB(t)
	account := walletdomain.Account{UserID: 7, Balance: money.FromDecimal(decimal.NewFromInt(1))}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	request := walletInvoiceRequest("NO-MONEY")
	err := New(db).CreateAndPayWithWallet(request, 7, nil)
	if !errors.Is(err, walletcontract.ErrInsufficientBalance) {
		t.Fatalf("error=%v, want insufficient balance", err)
	}
	var requests, entries int64
	_ = db.Model(&domain.Request{}).Count(&requests).Error
	_ = db.Model(&walletdomain.Transaction{}).Count(&entries).Error
	if err := db.First(&account, account.ID).Error; err != nil {
		t.Fatal(err)
	}
	if requests != 0 || entries != 0 || account.Balance.String() != "1.00" {
		t.Fatalf("insufficient payment was not atomic: requests=%d entries=%d balance=%s", requests, entries, account.Balance.String())
	}
}
