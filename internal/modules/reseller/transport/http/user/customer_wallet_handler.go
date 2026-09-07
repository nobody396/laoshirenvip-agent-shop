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

type UserCustomerPriceService interface {
	List(ownerUserID, customerUserID uint) (*resellerapp.CustomerPriceListResult, error)
	Set(ownerUserID, customerUserID, productID, skuID uint, fixedPrice decimal.Decimal) (*resellerapp.CustomerPriceQuote, error)
	Delete(ownerUserID, customerUserID, productID, skuID uint) error
}

type UserCustomerPriceHandler struct{ service UserCustomerPriceService }

func NewUserCustomerPriceHandler(service UserCustomerPriceService) *UserCustomerPriceHandler {
	if service == nil {
		panic("reseller customer price handler: service is nil")
	}
	return &UserCustomerPriceHandler{service: service}
}

type customerPriceRequest struct {
	FixedPriceAmount string `json:"fixed_price_amount" binding:"required"`
}

func (h *UserCustomerPriceHandler) List(c *gin.Context) {
	ownerUserID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	customerUserID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return
	}
	result, err := h.service.List(ownerUserID, customerUserID)
	if err != nil {
		respondCustomerPriceError(c, err)
		return
	}
	response.Success(c, gin.H{
		"customer": gin.H{
			"id":           result.Customer.ID,
			"email":        result.Customer.Email,
			"display_name": result.Customer.DisplayName,
			"status":       result.Customer.Status,
		},
		"settings": result.Settings,
	})
}

func (h *UserCustomerPriceHandler) Set(c *gin.Context) {
	ownerUserID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	customerUserID, productID, skuID, ok := parseCustomerPriceScope(c)
	if !ok {
		return
	}
	var req customerPriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	price, err := decimal.NewFromString(strings.TrimSpace(req.FixedPriceAmount))
	if err != nil || price.LessThanOrEqual(decimal.Zero) || !price.Equal(price.Round(2)) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	quote, err := h.service.Set(ownerUserID, customerUserID, productID, skuID, price)
	if err != nil {
		respondCustomerPriceError(c, err)
		return
	}
	response.Success(c, gin.H{
		"setting":             quote.Setting,
		"base_price_amount":   quote.BasePrice.StringFixed(2),
		"retail_price_amount": quote.RetailPrice.StringFixed(2),
		"fixed_price_amount":  quote.SpecialPrice.StringFixed(2),
		"gross_profit_amount": quote.GrossProfit.StringFixed(2),
	})
}

func (h *UserCustomerPriceHandler) Delete(c *gin.Context) {
	ownerUserID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	customerUserID, productID, skuID, ok := parseCustomerPriceScope(c)
	if !ok {
		return
	}
	if err := h.service.Delete(ownerUserID, customerUserID, productID, skuID); err != nil {
		respondCustomerPriceError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func parseCustomerPriceScope(c *gin.Context) (uint, uint, uint, bool) {
	customerUserID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.user_id_invalid", nil)
		return 0, 0, 0, false
	}
	productID, err := ginutil.ParseParamUint(c, "product_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return 0, 0, 0, false
	}
	skuID, err := ginutil.ParseParamUint(c, "sku_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return 0, 0, 0, false
	}
	return customerUserID, productID, skuID, true
}

func respondCustomerPriceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, resellercontract.ErrCustomerNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
	case errors.Is(err, resellercontract.ErrNotOpened), errors.Is(err, resellercontract.ErrProfileInactive):
		ginutil.RespondError(c, response.CodeForbidden, "error.forbidden", nil)
	case errors.Is(err, resellercontract.ErrPriceBelowBase):
		ginutil.RespondError(c, response.CodeBadRequest, "error.reseller_customer_price_below_base", nil)
	case errors.Is(err, resellercontract.ErrCustomerPriceAboveRetail):
		ginutil.RespondError(c, response.CodeBadRequest, "error.reseller_customer_price_above_retail", nil)
	case errors.Is(err, resellercontract.ErrCustomerPriceInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.reseller_customer_price_invalid", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
	}
}
