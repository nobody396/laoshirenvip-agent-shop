package application

import (
	"fmt"
	"strings"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

// AccountingLedgerService 分销利润入账、退款扣减与到期确认用例。
type AccountingLedgerService struct {
	store       resellercontract.AccountingLedgerStore
	confirmDays int
}

func NewAccountingLedgerService(store resellercontract.AccountingLedgerStore, confirmDays int) *AccountingLedgerService {
	const maxConfirmDays = 3650
	if confirmDays < 0 {
		confirmDays = 0
	}
	if confirmDays > maxConfirmDays {
		confirmDays = maxConfirmDays
	}
	return &AccountingLedgerService{store: store, confirmDays: confirmDays}
}

// PostOrderProfit 在调用方已开启的事务 store 上写入订单利润流水。
func (s *AccountingLedgerService) PostOrderProfit(store resellercontract.AccountingLedgerStore, order *orderdomain.Order, payment *paymentdomain.Payment) error {
	if s == nil || store == nil || order == nil || order.ID == 0 {
		return nil
	}
	if order.ResellerID == nil || *order.ResellerID == 0 {
		return nil
	}
	snapshot, err := store.GetOrderSnapshotByOrderID(order.ID)
	if err != nil {
		return err
	}
	if snapshot == nil {
		logger.Warnw("reseller_accounting_missing_snapshot_skip", "order_id", order.ID, "order_no", order.OrderNo)
		return nil
	}
	if !snapshot.ProfitEligible {
		return nil
	}
	grossProfit := snapshot.ProfitAmount.Decimal.Round(2)
	if grossProfit.LessThanOrEqual(decimal.Zero) {
		return nil
	}
	paymentFee := decimal.Zero
	if payment != nil && payment.FeePolicy == constants.PaymentFeePolicyMerchantAbsorbed {
		paymentFee = payment.FeeAmount.Decimal.Round(2)
		if paymentFee.LessThan(decimal.Zero) {
			paymentFee = decimal.Zero
		}
	}
	profit := grossProfit.Sub(paymentFee).Round(2)
	walletFee := decimal.Zero
	if profit.LessThan(decimal.Zero) {
		walletFee = profit.Abs().Round(2)
		profit = decimal.Zero
	}
	now := time.Now()
	availableAt := now.AddDate(0, 0, s.confirmDays)
	orderID := order.ID
	metadata := jsonmap.JSON{
		"order_no":                  order.OrderNo,
		"reseller_domain":           snapshot.Domain,
		"gross_profit_amount":       grossProfit.StringFixed(2),
		"payment_fee_amount":        paymentFee.StringFixed(2),
		"payment_fee_wallet_amount": walletFee.StringFixed(2),
		"net_profit_amount":         profit.StringFixed(2),
		"wallet_paid_amount":        order.WalletPaidAmount.String(),
		"online_paid_amount":        order.OnlinePaidAmount.String(),
		"snapshot_id":               snapshot.ID,
		"profit_block_reason":       snapshot.ProfitBlockReason,
	}
	if payment != nil {
		metadata["payment_id"] = payment.ID
		metadata["payment_channel_id"] = payment.ChannelID
		metadata["payment_amount"] = payment.Amount.String()
		metadata["payment_status"] = payment.Status
		metadata["payment_fee_policy"] = payment.FeePolicy
		metadata["payment_fee_rate"] = payment.FeeRate.String()
		metadata["payment_fixed_fee"] = payment.FixedFee.String()
	}
	entry := &resellerdomain.LedgerEntry{
		ResellerID:     snapshot.ResellerID,
		OrderID:        &orderID,
		Type:           resellerdomain.LedgerTypeOrderProfit,
		Amount:         money.FromDecimal(profit),
		Currency:       strings.TrimSpace(snapshot.Currency),
		IdempotencyKey: fmt.Sprintf("order_profit:%d", order.ID),
		MetadataJSON:   metadata,
		Status:         resellerdomain.LedgerStatusPendingConfirm,
		AvailableAt:    &availableAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if entry.Currency == "" {
		entry.Currency = strings.TrimSpace(order.Currency)
	}
	if entry.Currency == "" {
		return resellercontract.ErrLedgerInvalidSnapshot
	}
	created, err := store.CreateLedgerEntryIfNotExists(entry)
	if err != nil {
		return err
	}
	if !created {
		return nil
	}
	return RefreshBalanceAccount(store, snapshot.ResellerID, entry.Currency, now)
}

// PostOrderProfitForOrder 在订单状态流转事务中入账分销利润。
// 订单用例不需要感知尚未迁移的支付持久化实体。
func (s *AccountingLedgerService) PostOrderProfitForOrder(store resellercontract.AccountingLedgerStore, order *orderdomain.Order) error {
	return s.PostOrderProfit(store, order, nil)
}

// ConfirmDueLedgerEntries 将到期待确认流水转为可用并刷新余额缓存。
func (s *AccountingLedgerService) ConfirmDueLedgerEntries(now time.Time) (int64, error) {
	if s == nil || s.store == nil {
		return 0, nil
	}
	var affected int64
	err := s.store.WithinLedgerTransaction(func(store resellercontract.AccountingLedgerStore) error {
		scopes, err := store.ListDueLedgerScopes(now)
		if err != nil {
			return err
		}
		marked, err := store.MarkDueLedgerEntriesAvailable(now)
		if err != nil {
			return err
		}
		affected = marked
		for _, scope := range scopes {
			if err := RefreshBalanceAccount(store, scope.ResellerID, scope.Currency, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return affected, nil
}
