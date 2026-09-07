package integrationtest

import (
	"errors"
	"fmt"
	"testing"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	usergormstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	resellerapp "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	resellergormstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletgormstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestCustomerWalletTopUpAllowsOnlyTheResellersRegisteredCustomer(t *testing.T) {
	dsn := fmt.Sprintf("file:reseller_customer_wallet_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&resellerdomain.Profile{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&walletdomain.ResellerAccount{},
		&walletdomain.ResellerTransaction{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	owner := userdomain.User{Email: "owner@example.com", PasswordHash: "hash", Status: "active"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	profile := resellerdomain.Profile{UserID: owner.ID, Status: resellerdomain.ProfileStatusActive, SettlementStatus: resellerdomain.SettlementStatusNormal}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile: %v", err)
	}
	otherResellerID := profile.ID + 1
	customer := userdomain.User{RegistrationResellerID: &profile.ID, Email: "buyer@example.com", PasswordHash: "hash", Status: "active"}
	outsider := userdomain.User{RegistrationResellerID: &otherResellerID, Email: "outsider@example.com", PasswordHash: "hash", Status: "active"}
	if err := db.Create(&customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if err := db.Create(&outsider).Error; err != nil {
		t.Fatalf("create outsider: %v", err)
	}
	walletRepo := walletgormstore.New(db)
	wallets := walletapp.NewService(walletapp.Options{Repository: walletRepo, Transactions: walletRepo})
	if err := db.Create(&walletdomain.Account{UserID: owner.ID, Balance: money.FromDecimal(decimal.NewFromInt(100))}).Error; err != nil {
		t.Fatalf("fund owner: %v", err)
	}
	service := resellerapp.NewCustomerWalletService(resellergormstore.New(db), usergormstore.New(db), wallets)

	result, err := service.TopUp(resellerapp.CustomerWalletTopUpInput{
		OwnerUserID: owner.ID, CustomerUserID: customer.ID,
		Amount: money.FromDecimal(decimal.NewFromInt(30)), Reference: "customer-topup:test-001",
	})
	if err != nil {
		t.Fatalf("top up customer: %v", err)
	}
	if result.CustomerWallet.Balance.String() != "30.00" || result.OwnerWallet.Balance.String() != "70.00" {
		t.Fatalf("unexpected transfer result: %+v", result)
	}
	if _, err := service.TopUp(resellerapp.CustomerWalletTopUpInput{
		OwnerUserID: owner.ID, CustomerUserID: outsider.ID,
		Amount: money.FromDecimal(decimal.NewFromInt(1)), Reference: "customer-topup:test-outsider",
	}); !errors.Is(err, resellercontract.ErrCustomerNotFound) {
		t.Fatalf("cross-reseller top up error = %v, want ErrCustomerNotFound", err)
	}

	rows, total, err := service.ListCustomers(owner.ID, 1, 20, "")
	if err != nil {
		t.Fatalf("list customers: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].User.ID != customer.ID || rows[0].WalletBalance.String() != "30.00" {
		t.Fatalf("customer list leaked or missed rows: total=%d rows=%+v", total, rows)
	}
}
