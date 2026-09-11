package invoicehttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/dujiao-next/internal/constants"
	invoiceapp "github.com/dujiao-next/internal/modules/invoice/application"
	"github.com/dujiao-next/internal/modules/invoice/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
)

type OrderQuery interface {
	GetOrderByUserOrderNoForTenant(resellercontract.TenantContext, string, uint) (*orderdomain.Order, error)
	GetOrderByGuestOrderNoForTenant(resellercontract.TenantContext, string, string, string) (*orderdomain.Order, error)
}

type Handler struct {
	service   *invoiceapp.Service
	orders    OrderQuery
	gmshop    GMShopOrderQuery
	recharges RechargeQuery
}

type GMShopOrderQuery interface {
	Lookup(context.Context, string, string) (money.Amount, error)
}

type RechargeQuery interface {
	GetRechargeOrderByRechargeNo(uint, string) (*walletdomain.RechargeOrder, error)
}

func NewHandler(service *invoiceapp.Service, orders OrderQuery, gmshop GMShopOrderQuery, recharges RechargeQuery) *Handler {
	if service == nil || orders == nil {
		panic("invoice handler: required dependency is nil")
	}
	return &Handler{service: service, orders: orders, gmshop: gmshop, recharges: recharges}
}

type createRequest struct {
	OrderNo        string `json:"order_no" binding:"required"`
	InvoiceType    string `json:"invoice_type" binding:"required"`
	BuyerTitle     string `json:"buyer_title" binding:"required"`
	TaxNumber      string `json:"tax_number" binding:"required"`
	CompanyAddress string `json:"company_address"`
	CompanyPhone   string `json:"company_phone"`
	BankName       string `json:"bank_name"`
	BankAccount    string `json:"bank_account"`
	RecipientEmail string `json:"recipient_email" binding:"required"`
	OrderEmail     string `json:"order_email"`
	PaymentMethod  string `json:"payment_method"`
}

func (h *Handler) CreateGMShop(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if h.gmshop == nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invoice_invalid", nil)
		return
	}
	amount, err := h.gmshop.Lookup(c.Request.Context(), req.OrderNo, req.OrderEmail)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invoice_order_ineligible", nil)
		return
	}
	request, err := h.service.Create(c.Request.Context(), invoiceapp.CreateInput{
		Source: "gmshop", SourceHost: "laoshirenvip.com", OriginalOrderNo: req.OrderNo, OriginalAmount: amount,
		InvoiceType: req.InvoiceType, BuyerTitle: req.BuyerTitle, TaxNumber: req.TaxNumber,
		CompanyAddress: req.CompanyAddress, CompanyPhone: req.CompanyPhone, BankName: req.BankName, BankAccount: req.BankAccount,
		RecipientEmail: req.RecipientEmail, ClientIP: c.ClientIP(), PaymentMethod: req.PaymentMethod,
	})
	if err != nil {
		respondInvoiceCreateError(c, err)
		return
	}
	response.Success(c, publicRequest(request))
}

func (h *Handler) CreateRecharge(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if h.recharges == nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invoice_invalid", nil)
		return
	}
	recharge, err := h.recharges.GetRechargeOrderByRechargeNo(userID, strings.TrimSpace(req.OrderNo))
	if err != nil || recharge == nil || recharge.Status != constants.WalletRechargeStatusSuccess || recharge.Currency != constants.SiteCurrencyDefault || !recharge.Amount.Decimal.IsPositive() {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invoice_order_ineligible", nil)
		return
	}
	host := tenant(c).Host
	if host == "" {
		host = resellercontract.NormalizeHost(c.Request.Host)
	}
	request, err := h.service.Create(c.Request.Context(), invoiceapp.CreateInput{
		Source: "dujiao_recharge", SourceHost: host, OriginalOrderNo: recharge.RechargeNo, OriginalAmount: recharge.Amount,
		InvoiceType: req.InvoiceType, BuyerTitle: req.BuyerTitle, TaxNumber: req.TaxNumber,
		CompanyAddress: req.CompanyAddress, CompanyPhone: req.CompanyPhone, BankName: req.BankName, BankAccount: req.BankAccount,
		RecipientEmail: req.RecipientEmail, ClientIP: c.ClientIP(), PaymentMethod: req.PaymentMethod,
		UserID: userID, WalletResellerID: walletResellerID(c),
	})
	if err != nil {
		respondInvoiceCreateError(c, err)
		return
	}
	response.Success(c, publicRequest(request))
}

func tenant(c *gin.Context) resellercontract.TenantContext {
	if value, ok := resellercontract.TenantFromContext(c.Request.Context()); ok {
		return value
	}
	return resellercontract.MainTenantContext(c.Request.Host)
}

func eligible(order *orderdomain.Order) bool {
	if order == nil || order.Currency != constants.SiteCurrencyDefault || !order.RefundedAmount.Decimal.IsZero() {
		return false
	}
	switch order.Status {
	case constants.OrderStatusPaid, constants.OrderStatusPartiallyDelivered, constants.OrderStatusDelivered, constants.OrderStatusCompleted:
		return true
	default:
		return false
	}
}

func (h *Handler) CreateUser(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	h.create(c, userID, func(orderNo string) (*orderdomain.Order, error) {
		return h.orders.GetOrderByUserOrderNoForTenant(tenant(c), orderNo, userID)
	})
}

func (h *Handler) CreateGuest(c *gin.Context) {
	email, password, ok := ginutil.GetGuestCredentials(c)
	if !ok || email == "" || password == "" {
		ginutil.RespondError(c, response.CodeUnauthorized, "error.unauthorized", nil)
		return
	}
	h.create(c, 0, func(orderNo string) (*orderdomain.Order, error) {
		return h.orders.GetOrderByGuestOrderNoForTenant(tenant(c), orderNo, email, password)
	})
}

func (h *Handler) PaymentCallback(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil || len(body) > 64<<10 {
		c.String(400, "fail")
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if err := c.Request.ParseForm(); err != nil {
		c.String(400, "fail")
		return
	}
	form := c.Request.PostForm
	if c.Request.Method == http.MethodGet {
		form = c.Request.Form
	}
	if _, _, err := h.service.HandlePaymentCallback(form, body); err != nil {
		c.String(400, "fail")
		return
	}
	c.String(200, "success")
}

func (h *Handler) create(c *gin.Context, userID uint, lookup func(string) (*orderdomain.Order, error)) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	order, err := lookup(strings.TrimSpace(req.OrderNo))
	if err != nil || !eligible(order) {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invoice_order_ineligible", err)
		return
	}
	requestTenant := tenant(c)
	host := requestTenant.Host
	if host == "" {
		host = resellercontract.NormalizeHost(c.Request.Host)
	}
	request, err := h.service.Create(c.Request.Context(), invoiceapp.CreateInput{
		Source: "dujiao", SourceHost: host, OriginalOrderID: &order.ID,
		OriginalOrderNo: order.OrderNo, OriginalAmount: order.TotalAmount,
		InvoiceType: req.InvoiceType, BuyerTitle: req.BuyerTitle, TaxNumber: req.TaxNumber,
		CompanyAddress: req.CompanyAddress, CompanyPhone: req.CompanyPhone,
		BankName: req.BankName, BankAccount: req.BankAccount,
		RecipientEmail: req.RecipientEmail, ClientIP: c.ClientIP(), PaymentMethod: req.PaymentMethod,
		UserID: userID, WalletResellerID: walletResellerID(c),
	})
	if err != nil {
		respondInvoiceCreateError(c, err)
		return
	}
	response.Success(c, publicRequest(request))
}

func walletResellerID(c *gin.Context) *uint {
	value := tenant(c).ResellerID
	if value == nil || *value == 0 {
		return nil
	}
	id := *value
	return &id
}

func respondInvoiceCreateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, invoiceapp.ErrAlreadyRequested):
		ginutil.RespondError(c, response.CodeConflict, "error.invoice_already_requested", nil)
	case errors.Is(err, invoiceapp.ErrWalletInsufficient):
		ginutil.RespondError(c, response.CodeBadRequest, "error.invoice_wallet_insufficient", nil)
	case errors.Is(err, invoiceapp.ErrInvalidInput):
		ginutil.RespondError(c, response.CodeBadRequest, "error.invoice_invalid", nil)
	case errors.Is(err, invoiceapp.ErrPaymentUnavailable):
		ginutil.RespondError(c, response.CodeBadRequest, "error.invoice_create_failed", err)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.invoice_create_failed", err)
	}
}

func (h *Handler) GetPublic(c *gin.Context) {
	request, err := h.service.Get(c.Param("request_no"))
	if err != nil || request == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.invoice_not_found", err)
		return
	}
	response.Success(c, publicRequest(request))
}

func publicRequest(request *domain.Request) gin.H {
	paymentMethod := domain.PaymentMethodAlipay
	if strings.HasSuffix(strings.TrimSpace(request.ProviderRef), ":wallet") {
		paymentMethod = domain.PaymentMethodWallet
	}
	return gin.H{
		"request_no": request.RequestNo, "status": request.Status,
		"original_order_no": request.OriginalOrderNo, "invoice_type": request.InvoiceType,
		"original_amount": request.OriginalAmount, "invoice_fee_amount": request.InvoiceFeeAmount,
		"invoice_total_amount": request.InvoiceTotalAmount, "payment_fee_rate": request.PaymentFeeRate,
		"payment_fee_amount": request.PaymentFeeAmount, "payment_amount": request.PaymentAmount,
		"payment_method": paymentMethod,
		"pay_url":        request.PayURL, "qr_code": request.QRCode,
	}
}
