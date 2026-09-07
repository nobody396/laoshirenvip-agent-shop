package contract

import (
	"time"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
)

type AccountListFilter struct {
	Page     int
	PageSize int
	UserID   uint
}

type TransactionListFilter struct {
	Page        int
	PageSize    int
	UserID      uint
	OrderID     uint
	Type        string
	Direction   string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

type RechargeListFilter struct {
	Page         int
	PageSize     int
	RechargeNo   string
	UserID       uint
	UserKeyword  string
	PaymentID    uint
	ChannelID    uint
	ProviderType string
	ChannelType  string
	Status       string
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
	PaidFrom     *time.Time
	PaidTo       *time.Time
}

type RechargeInput struct {
	UserID   uint
	Amount   money.Amount
	Currency string
	Remark   string
}

type AdjustBalanceInput struct {
	UserID          uint
	OperatorAdminID uint
	Delta           money.Amount
	Currency        string
	Remark          string
}

type CreditInput struct {
	UserID    uint
	Amount    money.Amount
	Currency  string
	Type      string
	Reference string
	Remark    string
	OrderID   *uint
}

type OrderBalanceInput struct {
	OrderID          uint
	UserID           uint
	TotalAmount      money.Amount
	WalletPaidAmount money.Amount
	Currency         string
	UseBalance       bool
}

type OrderReleaseInput struct {
	OrderID          uint
	UserID           uint
	WalletPaidAmount money.Amount
	TotalAmount      money.Amount
	Currency         string
	TransactionType  string
	Remark           string
}

type ResellerAccountListFilter struct {
	Page       int
	PageSize   int
	ResellerID uint
	UserID     uint
}

type ResellerTransactionListFilter struct {
	Page       int
	PageSize   int
	ResellerID uint
	UserID     uint
	OrderID    uint
	Type       string
	Direction  string
}

type ResellerTransferInput struct {
	ResellerID     uint
	OwnerUserID    uint
	CustomerUserID uint
	Amount         money.Amount
	Currency       string
	Reference      string
	Remark         string
}

type ResellerTransferResult struct {
	OwnerAccount        *walletdomain.Account
	ResellerAccount     *walletdomain.ResellerAccount
	OwnerTransaction    *walletdomain.Transaction
	ResellerTransaction *walletdomain.ResellerTransaction
	AlreadyApplied      bool
}

type ResellerOrderBalanceInput struct {
	ResellerID       uint
	OrderID          uint
	UserID           uint
	TotalAmount      money.Amount
	WalletPaidAmount money.Amount
	Currency         string
	UseBalance       bool
}

type ResellerOrderReleaseInput struct {
	ResellerID       uint
	OrderID          uint
	UserID           uint
	WalletPaidAmount money.Amount
	TotalAmount      money.Amount
	Currency         string
	TransactionType  string
	Remark           string
}

type ResellerCreditInput struct {
	ResellerID uint
	UserID     uint
	Amount     money.Amount
	Currency   string
	Type       string
	Reference  string
	Remark     string
	OrderID    *uint
}
