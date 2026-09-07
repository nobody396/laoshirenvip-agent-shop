package integrationtest

import (
	"errors"
	"testing"

	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestTransferToResellerAccountMovesOwnerFundsOnce(t *testing.T) {
	repo, db := setupWalletRepositoryTest(t)
	if err := db.AutoMigrate(
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&walletdomain.ResellerAccount{},
		&walletdomain.ResellerTransaction{},
	); err != nil {
		t.Fatalf("migrate wallet tables: %v", err)
	}
	owner := walletdomain.Account{UserID: 11, Balance: money.FromDecimal(decimal.NewFromInt(100))}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner wallet: %v", err)
	}
	service := walletapp.NewService(walletapp.Options{Repository: repo, Transactions: repo})
	input := walletcontract.ResellerTransferInput{
		ResellerID:     7,
		OwnerUserID:    owner.UserID,
		CustomerUserID: 22,
		Amount:         money.FromDecimal(decimal.NewFromInt(30)),
		Currency:       "CNY",
		Reference:      "reseller-topup:test-001",
		Remark:         "线下收款充值",
	}

	first, err := service.TransferToResellerAccount(input)
	if err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	second, err := service.TransferToResellerAccount(input)
	if err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if !second.AlreadyApplied {
		t.Fatal("retry should report already applied")
	}

	ownerAfter, err := repo.GetAccountByUserID(owner.UserID)
	if err != nil {
		t.Fatalf("reload owner wallet: %v", err)
	}
	if ownerAfter == nil || ownerAfter.Balance.String() != "70.00" {
		t.Fatalf("owner balance = %+v, want 70.00", ownerAfter)
	}
	resellerAfter, err := repo.GetResellerAccount(7, 22)
	if err != nil {
		t.Fatalf("reload reseller wallet: %v", err)
	}
	if resellerAfter == nil || resellerAfter.Balance.String() != "30.00" {
		t.Fatalf("reseller balance = %+v, want 30.00", resellerAfter)
	}
	if first.OwnerTransaction == nil || first.ResellerTransaction == nil {
		t.Fatalf("missing transfer transactions: %+v", first)
	}
	if second.ResellerTransaction.ID != first.ResellerTransaction.ID {
		t.Fatalf("retry returned a different transaction: first=%d second=%d", first.ResellerTransaction.ID, second.ResellerTransaction.ID)
	}
}

func TestTransferToResellerAccountRollsBackInsufficientAndConflictingRequests(t *testing.T) {
	repo, db := setupWalletRepositoryTest(t)
	if err := db.AutoMigrate(&walletdomain.ResellerAccount{}, &walletdomain.ResellerTransaction{}); err != nil {
		t.Fatalf("migrate reseller wallet: %v", err)
	}
	owner := walletdomain.Account{UserID: 31, Balance: money.FromDecimal(decimal.NewFromInt(50))}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner wallet: %v", err)
	}
	service := walletapp.NewService(walletapp.Options{Repository: repo, Transactions: repo})
	base := walletcontract.ResellerTransferInput{
		ResellerID: 4, OwnerUserID: 31, CustomerUserID: 32,
		Currency: "CNY", Reference: "reseller-topup:test-rollback",
	}
	insufficient := base
	insufficient.Amount = money.FromDecimal(decimal.NewFromInt(80))
	if _, err := service.TransferToResellerAccount(insufficient); !errors.Is(err, walletcontract.ErrInsufficientBalance) {
		t.Fatalf("insufficient transfer error = %v", err)
	}
	ownerAfter, _ := repo.GetAccountByUserID(31)
	resellerAfter, _ := repo.GetResellerAccount(4, 32)
	if ownerAfter.Balance.String() != "50.00" || resellerAfter != nil {
		t.Fatalf("insufficient transfer mutated balances: owner=%+v tenant=%+v", ownerAfter, resellerAfter)
	}

	base.Amount = money.FromDecimal(decimal.NewFromInt(20))
	if _, err := service.TransferToResellerAccount(base); err != nil {
		t.Fatalf("valid transfer: %v", err)
	}
	conflict := base
	conflict.Amount = money.FromDecimal(decimal.NewFromInt(21))
	if _, err := service.TransferToResellerAccount(conflict); !errors.Is(err, walletcontract.ErrReferenceConflict) {
		t.Fatalf("reference conflict error = %v", err)
	}
	ownerAfter, _ = repo.GetAccountByUserID(31)
	resellerAfter, _ = repo.GetResellerAccount(4, 32)
	if ownerAfter.Balance.String() != "30.00" || resellerAfter.Balance.String() != "20.00" {
		t.Fatalf("conflict changed balances: owner=%s tenant=%s", ownerAfter.Balance.String(), resellerAfter.Balance.String())
	}
}
