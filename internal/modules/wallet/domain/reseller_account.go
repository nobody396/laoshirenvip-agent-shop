package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// ResellerAccount is a customer balance owned by one reseller tenant. It is
// deliberately separate from the platform-wide purchasing wallet.
type ResellerAccount struct {
	ID         uint         `gorm:"primarykey" json:"id"`
	ResellerID uint         `gorm:"not null;uniqueIndex:idx_reseller_wallet_owner" json:"reseller_id"`
	UserID     uint         `gorm:"not null;uniqueIndex:idx_reseller_wallet_owner" json:"user_id"`
	Balance    money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"balance"`
	CreatedAt  time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt  time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt  *time.Time   `gorm:"index" json:"-"`
}

func (ResellerAccount) TableName() string { return "reseller_wallet_accounts" }

// ResellerTransaction is the immutable tenant-wallet ledger.
type ResellerTransaction struct {
	ID             uint         `gorm:"primarykey" json:"id"`
	ResellerID     uint         `gorm:"not null;index" json:"reseller_id"`
	UserID         uint         `gorm:"not null;index" json:"user_id"`
	OperatorUserID *uint        `gorm:"index" json:"operator_user_id,omitempty"`
	OrderID        *uint        `gorm:"index" json:"order_id,omitempty"`
	Type           string       `gorm:"type:varchar(40);not null;index" json:"type"`
	Direction      string       `gorm:"type:varchar(16);not null;index" json:"direction"`
	Amount         money.Amount `gorm:"type:decimal(20,2);not null" json:"amount"`
	BalanceBefore  money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"balance_before"`
	BalanceAfter   money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"balance_after"`
	Currency       string       `gorm:"type:varchar(16);not null;default:'CNY'" json:"currency"`
	Reference      string       `gorm:"type:varchar(160);not null;uniqueIndex" json:"reference"`
	Remark         string       `gorm:"type:varchar(255)" json:"remark"`
	CreatedAt      time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt      time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt      *time.Time   `gorm:"index" json:"-"`
}

func (ResellerTransaction) TableName() string { return "reseller_wallet_transactions" }
