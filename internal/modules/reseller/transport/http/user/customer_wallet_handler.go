package userhttp

import (
	"errors"
	"strings"
	"time"

	resellerapp "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type UserCustomerWalletService interface {
	TopUp(input resellerapp.CustomerWalletTopUpInput) (*resellerapp.CustomerWalletTopUpResult, error)
	ListCustomers(ownerUserID uint, page, pageSize int, keyword string) ([]resellerapp.CustomerWalletListRow, int64, error)
	ListCustomerTransactions(ownerUserID, customerUserID uint, page, pageSize int) ([]walletdomain.ResellerTransaction, int64, error)
}

type UserCustomerWalletHandler struct{ service UserCustomerWalletService }

func NewUserCustomerWalletHandler(service UserCustomerWalletService) *UserCustomerWalletHandler {
	if service == nil {
		panic("reseller customer wallet handler: service is nil")
	}
	return &UserCustomerWalletHandler{service: service}
}

type customerWalletTopUpRequest struct {
	Amount    string `json:"amount" binding:"required"`
	RequestID string `json:"request_id" binding:"required"`
	Remark    string `json:"remark"`
}

type customerWalletUserResponse struct {
	ID          uint         `json:"id"`
	Email       string       `json:"email"`
	DisplayName string       `json:"display_name"`
	Status      string       `json:"status"`
	Balance     money.Amount `json:"wallet_balance"`
}

type customerWalletTransactionResponse struct {
	ID            uint         `json:"id"`
	Type          string       `json:"type"`
	Direction     string       `json:"direction"`
	Amount        money.Amount `json:"amount"`
	BalanceBefore money.Amount `json:"balance_before"`
	BalanceAfter  money.Amount `json:"balance_after"`
	Remark        string       `json:"remark"`
	CreatedAt     time.Time    `json:"created_at"`
}

func (h *UserCustomerWalletHandler) ListCustomers(c *gin.Context) {
	ownerUserID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	rows, total, err := h.service.ListCustomers(ownerUserID, page, pageSize, strings.TrimSpace(c.Query("keyword")))
	if err != nil {
		respondCustomerWalletError(c, err)
		return
	}
	result := make([]customerWalletUserResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, customerWalletUserResponse{
			ID: row.User.ID, Email: row.User.Email, DisplayName: row.User.DisplayName,
			Status: row.User.Status, Balance: row.WalletBalance,
		})
	}
	response.SuccessWithPage(c, result, response.BuildPagination(page, pageSize, total))
}

func (h *UserCustomerWalletHandler) TopUp(c *gin.Context) {
	ownerUserID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	customerUserID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}
	var req customerWalletTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil || amount.LessThanOrEqual(decimal.Zero) || !amount.Equal(amount.Round(2)) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	reference := strings.TrimSpace(req.RequestID)
	if len(reference) < 8 || len(reference) > 140 || strings.ContainsAny(reference, " \t\r\n") {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	result, err := h.service.TopUp(resellerapp.CustomerWalletTopUpInput{
		OwnerUserID: ownerUserID, CustomerUserID: customerUserID,
		Amount: money.FromDecimal(amount), Reference: reference, Remark: strings.TrimSpace(req.Remark),
	})
	if err != nil {
		respondCustomerWalletError(c, err)
		return
	}
	txn := result.CustomerTransaction
	response.Success(c, gin.H{
		"transaction_id":            txn.ID,
		"amount":                    txn.Amount,
		"balance_before":            txn.BalanceBefore,
		"balance_after":             result.CustomerWallet.Balance,
		"transaction_balance_after": txn.BalanceAfter,
		"owner_wallet_balance":      result.OwnerWallet.Balance,
		"already_applied":           result.AlreadyApplied,
	})
}

func (h *UserCustomerWalletHandler) ListTransactions(c *gin.Context) {
	ownerUserID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	customerUserID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	rows, total, err := h.service.ListCustomerTransactions(ownerUserID, customerUserID, page, pageSize)
	if err != nil {
		respondCustomerWalletError(c, err)
		return
	}
	result := make([]customerWalletTransactionResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, customerWalletTransactionResponse{
			ID: row.ID, Type: row.Type, Direction: row.Direction, Amount: row.Amount,
			BalanceBefore: row.BalanceBefore, BalanceAfter: row.BalanceAfter,
			Remark: row.Remark, CreatedAt: row.CreatedAt,
		})
	}
	response.SuccessWithPage(c, result, response.BuildPagination(page, pageSize, total))
}

func respondCustomerWalletError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, resellercontract.ErrCustomerNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
	case errors.Is(err, resellercontract.ErrNotOpened), errors.Is(err, resellercontract.ErrProfileInactive), errors.Is(err, resellercontract.ErrSettlementUnavailable):
		ginutil.RespondError(c, response.CodeForbidden, "error.forbidden", nil)
	case errors.Is(err, walletcontract.ErrInvalidAmount), errors.Is(err, walletcontract.ErrReferenceConflict):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
	case errors.Is(err, walletcontract.ErrInsufficientBalance):
		ginutil.RespondError(c, response.CodeBadRequest, "error.wallet_insufficient_balance", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
	}
}
