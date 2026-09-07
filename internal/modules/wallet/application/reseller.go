package application

import (
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

const maxResellerTransferReferenceLength = 140

func (s *Service) GetResellerAccount(resellerID, userID uint) (*walletdomain.ResellerAccount, error) {
	if resellerID == 0 || userID == 0 {
		return nil, walletcontract.ErrAccountNotFound
	}
	account, err := s.repository.GetResellerAccount(resellerID, userID)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return &walletdomain.ResellerAccount{
			ResellerID: resellerID,
			UserID:     userID,
			Balance:    money.FromDecimal(decimal.Zero),
		}, nil
	}
	return account, nil
}

func (s *Service) GetResellerBalancesByUserIDs(resellerID uint, userIDs []uint) (map[uint]money.Amount, error) {
	result := make(map[uint]money.Amount, len(userIDs))
	if resellerID == 0 || len(userIDs) == 0 {
		return result, nil
	}
	accounts, err := s.repository.GetResellerAccountsByUserIDs(resellerID, userIDs)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		result[account.UserID] = account.Balance
	}
	return result, nil
}

func (s *Service) ListResellerTransactions(filter walletcontract.ResellerTransactionListFilter) ([]walletdomain.ResellerTransaction, int64, error) {
	return s.repository.ListResellerTransactions(filter)
}

func (s *Service) TransferToResellerAccount(input walletcontract.ResellerTransferInput) (*walletcontract.ResellerTransferResult, error) {
	amount := input.Amount.Decimal.Round(2)
	reference := strings.TrimSpace(input.Reference)
	if input.ResellerID == 0 || input.OwnerUserID == 0 || input.CustomerUserID == 0 || input.OwnerUserID == input.CustomerUserID {
		return nil, walletcontract.ErrAccountNotFound
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, walletcontract.ErrInvalidAmount
	}
	if reference == "" || len(reference) > maxResellerTransferReferenceLength {
		return nil, walletcontract.ErrReferenceConflict
	}
	if s.transactions == nil {
		return nil, walletcontract.ErrTransactionRequired
	}
	currency := normalizeCurrency(input.Currency)
	remark := cleanRemark(input.Remark, "代理给子站用户充值")
	ownerReference := reference + ":owner"
	result := &walletcontract.ResellerTransferResult{}
	err := s.transactions.WithinTransaction(func(tx walletcontract.Transaction) error {
		repository := tx.Wallets()
		existing, err := repository.GetResellerTransactionByReference(reference)
		if err != nil {
			return err
		}
		if existing != nil {
			if existing.ResellerID != input.ResellerID || existing.UserID != input.CustomerUserID ||
				existing.Amount.Decimal.Round(2).Cmp(amount) != 0 || normalizeCurrency(existing.Currency) != currency {
				return walletcontract.ErrReferenceConflict
			}
			ownerTransaction, err := repository.GetTransactionByReference(ownerReference)
			if err != nil {
				return err
			}
			ownerAccount, err := repository.GetAccountByUserID(input.OwnerUserID)
			if err != nil {
				return err
			}
			resellerAccount, err := repository.GetResellerAccount(input.ResellerID, input.CustomerUserID)
			if err != nil {
				return err
			}
			if ownerTransaction == nil || ownerAccount == nil || resellerAccount == nil {
				return walletcontract.ErrTransactionCreateFailed
			}
			if ownerTransaction.UserID != input.OwnerUserID || ownerTransaction.Direction != constants.WalletTxnDirectionOut ||
				ownerTransaction.Amount.Decimal.Round(2).Cmp(amount) != 0 {
				return walletcontract.ErrReferenceConflict
			}
			result.OwnerAccount = ownerAccount
			result.ResellerAccount = resellerAccount
			result.OwnerTransaction = ownerTransaction
			result.ResellerTransaction = existing
			result.AlreadyApplied = true
			return nil
		}

		ownerAccount, err := repository.GetAccountByUserIDForUpdate(input.OwnerUserID)
		if err != nil {
			return err
		}
		if ownerAccount == nil || ownerAccount.Balance.Decimal.Round(2).LessThan(amount) {
			return walletcontract.ErrInsufficientBalance
		}
		now := time.Now()
		resellerAccount, err := repository.GetResellerAccountForUpdate(input.ResellerID, input.CustomerUserID)
		if err != nil {
			return err
		}
		if resellerAccount == nil {
			resellerAccount = &walletdomain.ResellerAccount{
				ResellerID: input.ResellerID,
				UserID:     input.CustomerUserID,
				Balance:    money.FromDecimal(decimal.Zero),
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if err := repository.CreateResellerAccount(resellerAccount); err != nil {
				return walletcontract.ErrAccountCreateFailed
			}
		}

		ownerBefore := ownerAccount.Balance.Decimal.Round(2)
		ownerAfter := ownerBefore.Sub(amount).Round(2)
		customerBefore := resellerAccount.Balance.Decimal.Round(2)
		customerAfter := customerBefore.Add(amount).Round(2)
		ownerAccount.Balance = money.FromDecimal(ownerAfter)
		ownerAccount.UpdatedAt = now
		resellerAccount.Balance = money.FromDecimal(customerAfter)
		resellerAccount.UpdatedAt = now
		if err := repository.UpdateAccount(ownerAccount); err != nil {
			return walletcontract.ErrAccountUpdateFailed
		}
		if err := repository.UpdateResellerAccount(resellerAccount); err != nil {
			return walletcontract.ErrAccountUpdateFailed
		}
		resellerID := input.ResellerID
		ownerTransaction := &walletdomain.Transaction{
			UserID: input.OwnerUserID, Type: constants.WalletTxnTypeResellerCustomerTopup,
			Direction: constants.WalletTxnDirectionOut, Amount: money.FromDecimal(amount),
			BalanceBefore: money.FromDecimal(ownerBefore), BalanceAfter: money.FromDecimal(ownerAfter),
			Currency: currency, Reference: ownerReference,
			Remark: fmt.Sprintf("子站 R#%d 用户充值：%s", resellerID, remark), CreatedAt: now, UpdatedAt: now,
		}
		operatorUserID := input.OwnerUserID
		resellerTransaction := &walletdomain.ResellerTransaction{
			ResellerID: input.ResellerID, UserID: input.CustomerUserID, OperatorUserID: &operatorUserID,
			Type: constants.WalletTxnTypeResellerCustomerTopup, Direction: constants.WalletTxnDirectionIn,
			Amount: money.FromDecimal(amount), BalanceBefore: money.FromDecimal(customerBefore),
			BalanceAfter: money.FromDecimal(customerAfter), Currency: currency,
			Reference: reference, Remark: remark, CreatedAt: now, UpdatedAt: now,
		}
		if err := repository.CreateTransaction(ownerTransaction); err != nil {
			return walletcontract.ErrTransactionCreateFailed
		}
		if err := repository.CreateResellerTransaction(resellerTransaction); err != nil {
			return walletcontract.ErrTransactionCreateFailed
		}
		result.OwnerAccount = ownerAccount
		result.ResellerAccount = resellerAccount
		result.OwnerTransaction = ownerTransaction
		result.ResellerTransaction = resellerTransaction
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func resellerOrderAllocationReference(repository walletcontract.Repository, orderID uint, action string) (string, error) {
	base := fmt.Sprintf("reseller-order:%d:%s", orderID, strings.TrimSpace(action))
	count, err := repository.CountResellerOrderTransactionsByType(orderID, action)
	if err != nil {
		return "", err
	}
	if count == 0 {
		return base, nil
	}
	return fmt.Sprintf("%s:%d", base, count+1), nil
}

func ensureResellerAccountForUpdate(repository walletcontract.Repository, resellerID, userID uint, now time.Time) (*walletdomain.ResellerAccount, error) {
	account, err := repository.GetResellerAccountForUpdate(resellerID, userID)
	if err != nil {
		return nil, err
	}
	if account != nil {
		return account, nil
	}
	account = &walletdomain.ResellerAccount{
		ResellerID: resellerID, UserID: userID,
		Balance: money.FromDecimal(decimal.Zero), CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateResellerAccount(account); err != nil {
		return nil, walletcontract.ErrAccountCreateFailed
	}
	return account, nil
}

func (s *Service) CreditResellerInTransaction(tx walletcontract.Transaction, input walletcontract.ResellerCreditInput) (*walletdomain.ResellerAccount, *walletdomain.ResellerTransaction, error) {
	if tx == nil {
		return nil, nil, walletcontract.ErrTransactionRequired
	}
	amount := input.Amount.Decimal.Round(2)
	reference := strings.TrimSpace(input.Reference)
	if input.ResellerID == 0 || input.UserID == 0 {
		return nil, nil, walletcontract.ErrAccountNotFound
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, nil, walletcontract.ErrInvalidAmount
	}
	if reference == "" {
		return nil, nil, walletcontract.ErrReferenceConflict
	}
	repository := tx.Wallets()
	existing, err := repository.GetResellerTransactionByReference(reference)
	if err != nil {
		return nil, nil, err
	}
	if existing != nil {
		if existing.ResellerID != input.ResellerID || existing.UserID != input.UserID || existing.Amount.Decimal.Round(2).Cmp(amount) != 0 {
			return nil, nil, walletcontract.ErrReferenceConflict
		}
		account, err := repository.GetResellerAccount(input.ResellerID, input.UserID)
		return account, existing, err
	}
	now := time.Now()
	account, err := ensureResellerAccountForUpdate(repository, input.ResellerID, input.UserID, now)
	if err != nil {
		return nil, nil, err
	}
	before := account.Balance.Decimal.Round(2)
	after := before.Add(amount).Round(2)
	account.Balance = money.FromDecimal(after)
	account.UpdatedAt = now
	if err := repository.UpdateResellerAccount(account); err != nil {
		return nil, nil, walletcontract.ErrAccountUpdateFailed
	}
	typeName := strings.TrimSpace(input.Type)
	if typeName == "" {
		typeName = constants.WalletTxnTypeAdminRefund
	}
	transaction := &walletdomain.ResellerTransaction{
		ResellerID: input.ResellerID, UserID: input.UserID, OrderID: input.OrderID,
		Type: typeName, Direction: constants.WalletTxnDirectionIn, Amount: money.FromDecimal(amount),
		BalanceBefore: money.FromDecimal(before), BalanceAfter: money.FromDecimal(after),
		Currency: normalizeCurrency(input.Currency), Reference: reference,
		Remark: cleanRemark(input.Remark, "子站钱包入账"), CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateResellerTransaction(transaction); err != nil {
		return nil, nil, walletcontract.ErrTransactionCreateFailed
	}
	return account, transaction, nil
}

func (s *Service) ApplyResellerOrderBalance(tx walletcontract.Transaction, input walletcontract.ResellerOrderBalanceInput) (money.Amount, error) {
	if tx == nil {
		return money.Amount{}, walletcontract.ErrTransactionRequired
	}
	if !input.UseBalance {
		return input.WalletPaidAmount, nil
	}
	if input.ResellerID == 0 || input.UserID == 0 {
		return money.Amount{}, walletcontract.ErrAccountNotFound
	}
	existingPaid := input.WalletPaidAmount.Decimal.Round(2)
	if existingPaid.GreaterThan(decimal.Zero) {
		return money.FromDecimal(existingPaid), nil
	}
	total := input.TotalAmount.Decimal.Round(2)
	if total.LessThanOrEqual(decimal.Zero) {
		return money.FromDecimal(decimal.Zero), nil
	}
	repository := tx.Wallets()
	now := time.Now()
	account, err := ensureResellerAccountForUpdate(repository, input.ResellerID, input.UserID, now)
	if err != nil {
		return money.Amount{}, err
	}
	available := account.Balance.Decimal.Round(2)
	if available.LessThanOrEqual(decimal.Zero) {
		return money.FromDecimal(decimal.Zero), nil
	}
	deduct := decimal.Min(available, total).Round(2)
	reference, err := resellerOrderAllocationReference(repository, input.OrderID, constants.WalletTxnTypeOrderPay)
	if err != nil {
		return money.Amount{}, err
	}
	existing, err := repository.GetResellerTransactionByReference(reference)
	if err != nil {
		return money.Amount{}, err
	}
	if existing != nil {
		return existing.Amount, nil
	}
	before := available
	after := before.Sub(deduct).Round(2)
	account.Balance = money.FromDecimal(after)
	account.UpdatedAt = now
	if err := repository.UpdateResellerAccount(account); err != nil {
		return money.Amount{}, walletcontract.ErrAccountUpdateFailed
	}
	orderID := input.OrderID
	transaction := &walletdomain.ResellerTransaction{
		ResellerID: input.ResellerID, UserID: input.UserID, OrderID: &orderID,
		Type: constants.WalletTxnTypeOrderPay, Direction: constants.WalletTxnDirectionOut,
		Amount: money.FromDecimal(deduct), BalanceBefore: money.FromDecimal(before),
		BalanceAfter: money.FromDecimal(after), Currency: normalizeCurrency(input.Currency),
		Reference: reference, Remark: "子站订单余额支付", CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateResellerTransaction(transaction); err != nil {
		return money.Amount{}, walletcontract.ErrTransactionCreateFailed
	}
	return transaction.Amount, nil
}

func (s *Service) ReleaseResellerOrderBalance(tx walletcontract.Transaction, input walletcontract.ResellerOrderReleaseInput, claim walletcontract.ReleaseClaim) (money.Amount, error) {
	if tx == nil || claim == nil {
		return money.Amount{}, walletcontract.ErrTransactionRequired
	}
	amount := input.WalletPaidAmount.Decimal.Round(2)
	if input.ResellerID == 0 || input.UserID == 0 || amount.LessThanOrEqual(decimal.Zero) {
		return money.FromDecimal(decimal.Zero), nil
	}
	if amount.GreaterThan(input.TotalAmount.Decimal.Round(2)) {
		return money.Amount{}, walletcontract.ErrRefundExceeded
	}
	repository := tx.Wallets()
	reference, err := resellerOrderAllocationReference(repository, input.OrderID, input.TransactionType)
	if err != nil {
		return money.Amount{}, err
	}
	existing, err := repository.GetResellerTransactionByReference(reference)
	if err != nil {
		return money.Amount{}, err
	}
	if existing != nil {
		return existing.Amount, nil
	}
	now := time.Now()
	claimed, err := claim(now)
	if err != nil {
		return money.Amount{}, err
	}
	if !claimed {
		return money.FromDecimal(decimal.Zero), nil
	}
	account, err := ensureResellerAccountForUpdate(repository, input.ResellerID, input.UserID, now)
	if err != nil {
		return money.Amount{}, err
	}
	before := account.Balance.Decimal.Round(2)
	after := before.Add(amount).Round(2)
	account.Balance = money.FromDecimal(after)
	account.UpdatedAt = now
	if err := repository.UpdateResellerAccount(account); err != nil {
		return money.Amount{}, walletcontract.ErrAccountUpdateFailed
	}
	orderID := input.OrderID
	transaction := &walletdomain.ResellerTransaction{
		ResellerID: input.ResellerID, UserID: input.UserID, OrderID: &orderID,
		Type: input.TransactionType, Direction: constants.WalletTxnDirectionIn,
		Amount: money.FromDecimal(amount), BalanceBefore: money.FromDecimal(before),
		BalanceAfter: money.FromDecimal(after), Currency: normalizeCurrency(input.Currency),
		Reference: reference, Remark: cleanRemark(input.Remark, "子站订单余额退回"),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateResellerTransaction(transaction); err != nil {
		return money.Amount{}, walletcontract.ErrTransactionCreateFailed
	}
	return transaction.Amount, nil
}

const (
	resellerPaymentFeeReservePrefix = "reseller-payment-fee-reserve:"
	resellerPaymentFeeReleasePrefix = "reseller-payment-fee-release:"
)

func resellerPaymentFeeReserveReference(paymentID uint) string {
	return fmt.Sprintf("%s%d", resellerPaymentFeeReservePrefix, paymentID)
}

func resellerPaymentFeeReleaseReference(paymentID uint) string {
	return fmt.Sprintf("%s%d", resellerPaymentFeeReleasePrefix, paymentID)
}

func paymentIDFromFeeReserveReference(reference string) (uint, bool) {
	var paymentID uint
	if _, err := fmt.Sscanf(strings.TrimSpace(reference), resellerPaymentFeeReservePrefix+"%d", &paymentID); err != nil || paymentID == 0 {
		return 0, false
	}
	return paymentID, true
}

// ReserveResellerPaymentFee debits the reseller owner's prepaid purchasing
// wallet for only the part of an absorbed gateway fee that the order margin
// cannot cover. The debit is the hold: success keeps it, while failed or
// expired payments create an equal release transaction.
func (s *Service) ReserveResellerPaymentFee(tx walletcontract.Transaction, input walletcontract.ResellerPaymentFeeReserveInput) (*walletdomain.Transaction, error) {
	if tx == nil {
		return nil, walletcontract.ErrTransactionRequired
	}
	amount := input.Amount.Decimal.Round(2)
	if input.OwnerUserID == 0 || input.OrderID == 0 || input.PaymentID == 0 {
		return nil, walletcontract.ErrAccountNotFound
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, walletcontract.ErrInvalidAmount
	}
	repository := tx.Wallets()
	reference := resellerPaymentFeeReserveReference(input.PaymentID)
	existing, err := repository.GetTransactionByReference(reference)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.UserID != input.OwnerUserID || existing.OrderID == nil || *existing.OrderID != input.OrderID ||
			existing.Type != constants.WalletTxnTypeResellerPaymentFeeReserve || existing.Direction != constants.WalletTxnDirectionOut ||
			existing.Amount.Decimal.Round(2).Cmp(amount) != 0 {
			return nil, walletcontract.ErrReferenceConflict
		}
		return existing, nil
	}

	now := time.Now()
	account, err := ensureAccountForUpdate(repository, input.OwnerUserID, now)
	if err != nil {
		return nil, err
	}
	before := account.Balance.Decimal.Round(2)
	if before.LessThan(amount) {
		return nil, walletcontract.ErrInsufficientBalance
	}
	after := before.Sub(amount).Round(2)
	account.Balance = money.FromDecimal(after)
	account.UpdatedAt = now
	if err := repository.UpdateAccount(account); err != nil {
		return nil, walletcontract.ErrAccountUpdateFailed
	}
	orderID := input.OrderID
	entry := &walletdomain.Transaction{
		UserID: input.OwnerUserID, OrderID: &orderID,
		Type: constants.WalletTxnTypeResellerPaymentFeeReserve, Direction: constants.WalletTxnDirectionOut,
		Amount: money.FromDecimal(amount), BalanceBefore: money.FromDecimal(before), BalanceAfter: money.FromDecimal(after),
		Currency: normalizeCurrency(input.Currency), Reference: reference,
		Remark: "子站在线支付手续费缺口暂扣", CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateTransaction(entry); err != nil {
		return nil, walletcontract.ErrTransactionCreateFailed
	}
	return entry, nil
}

// ReleaseResellerPaymentFee returns one fee hold exactly once.
func (s *Service) ReleaseResellerPaymentFee(tx walletcontract.Transaction, paymentID uint) (*walletdomain.Transaction, error) {
	if tx == nil {
		return nil, walletcontract.ErrTransactionRequired
	}
	if paymentID == 0 {
		return nil, nil
	}
	repository := tx.Wallets()
	reserve, err := repository.GetTransactionByReference(resellerPaymentFeeReserveReference(paymentID))
	if err != nil || reserve == nil {
		return nil, err
	}
	releaseReference := resellerPaymentFeeReleaseReference(paymentID)
	existing, err := repository.GetTransactionByReference(releaseReference)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if existing.UserID != reserve.UserID || existing.Type != constants.WalletTxnTypeResellerPaymentFeeRelease ||
			existing.Direction != constants.WalletTxnDirectionIn || existing.Amount.Decimal.Round(2).Cmp(reserve.Amount.Decimal.Round(2)) != 0 {
			return nil, walletcontract.ErrReferenceConflict
		}
		return existing, nil
	}

	now := time.Now()
	account, err := ensureAccountForUpdate(repository, reserve.UserID, now)
	if err != nil {
		return nil, err
	}
	amount := reserve.Amount.Decimal.Round(2)
	before := account.Balance.Decimal.Round(2)
	after := before.Add(amount).Round(2)
	account.Balance = money.FromDecimal(after)
	account.UpdatedAt = now
	if err := repository.UpdateAccount(account); err != nil {
		return nil, walletcontract.ErrAccountUpdateFailed
	}
	entry := &walletdomain.Transaction{
		UserID: reserve.UserID, OrderID: reserve.OrderID,
		Type: constants.WalletTxnTypeResellerPaymentFeeRelease, Direction: constants.WalletTxnDirectionIn,
		Amount: money.FromDecimal(amount), BalanceBefore: money.FromDecimal(before), BalanceAfter: money.FromDecimal(after),
		Currency: normalizeCurrency(reserve.Currency), Reference: releaseReference,
		Remark: "子站在线支付未成功，退回手续费暂扣", CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateTransaction(entry); err != nil {
		return nil, walletcontract.ErrTransactionCreateFailed
	}
	return entry, nil
}

// ReleaseResellerPaymentFeesForOrder releases every hold for the order except
// the payment that remains authoritative after a successful link replacement.
func (s *Service) ReleaseResellerPaymentFeesForOrder(tx walletcontract.Transaction, orderID, exceptPaymentID uint) error {
	if tx == nil {
		return walletcontract.ErrTransactionRequired
	}
	if orderID == 0 {
		return nil
	}
	rows, _, err := tx.Wallets().ListTransactions(walletcontract.TransactionListFilter{
		OrderID: orderID,
		Type:    constants.WalletTxnTypeResellerPaymentFeeReserve,
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		paymentID, ok := paymentIDFromFeeReserveReference(row.Reference)
		if !ok || paymentID == exceptPaymentID {
			continue
		}
		if _, err := s.ReleaseResellerPaymentFee(tx, paymentID); err != nil {
			return err
		}
	}
	return nil
}
