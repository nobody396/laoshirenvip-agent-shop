package sharedstockhttp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	downstreamcallbackdomain "github.com/dujiao-next/internal/modules/downstreamcallback/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	upstreamhttp "github.com/dujiao-next/internal/modules/upstreamapi/transport/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Trade(c *gin.Context) {
	uid, cid := userID(c), credentialID(c)
	if uid == 0 || cid == 0 {
		failure(c, "商户ID不存在")
		return
	}
	requestNo := strings.TrimSpace(c.PostForm("request_no"))
	if requestNo == "" || len(requestNo) > 64 {
		failure(c, "request_no不能为空且不能超过64字符")
		return
	}
	if existing, _ := h.DownstreamRefs.GetByCredentialAndDownstreamNo(cid, requestNo); existing != nil {
		order, err := h.Orders.GetOrderByUser(existing.OrderID, uid)
		if err != nil || order == nil {
			failure(c, "订单查询失败")
			return
		}
		h.tradeResponse(c, order)
		return
	}

	product, err := h.productByCode(c.PostForm("shared_code"))
	if err != nil || product == nil || !product.IsActive {
		failure(c, "商品不存在")
		return
	}
	sku, err := h.selectSKU(*product, c.PostForm("race"))
	if err != nil {
		failure(c, "商品规格不存在")
		return
	}
	quantity, _ := strconv.Atoi(c.PostForm("num"))
	if quantity < 1 {
		failure(c, "购买数量错误")
		return
	}

	order, err := h.Orders.CreateOrder(upstreamhttp.CreateOrderInput{
		UserID: uid, ClientIP: c.ClientIP(), SkipRiskControl: true,
		Items: []upstreamhttp.CreateOrderItem{{
			ProductID: product.ID, SKUID: sku.ID, Quantity: quantity, FulfillmentType: product.FulfillmentType,
		}},
	})
	if err != nil {
		failure(c, sharedOrderError(err))
		return
	}
	ref := &downstreamcallbackdomain.OrderRef{
		OrderID: order.ID, ApiCredentialID: cid, DownstreamOrderNo: requestNo,
		TraceID: requestNo, CallbackStatus: downstreamcallbackdomain.StatusPending,
	}
	if err := h.DownstreamRefs.Create(ref); err != nil {
		_, _ = h.Orders.CancelOrder(order.ID, uid)
		failure(c, "订单幂等记录创建失败")
		return
	}
	paid, err := h.Payments.CreatePayment(upstreamhttp.CreatePaymentInput{OrderID: order.ID, UseBalance: true, ClientIP: c.ClientIP()})
	if err != nil || paid == nil || !paid.OrderPaid {
		_, _ = h.Orders.CancelOrder(order.ID, uid)
		failure(c, "余额不足或付款失败")
		return
	}
	h.tradeResponse(c, h.waitForOrder(uid, order.ID, 25*time.Second))
}

func (h *Handler) Query(c *gin.Context) {
	raw := strings.TrimSpace(c.Param("tradeNo"))
	if raw == "" {
		raw = strings.TrimSpace(c.PostForm("tradeNo"))
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		failure(c, "订单不存在")
		return
	}
	order, err := h.Orders.GetOrderByUser(uint(id), userID(c))
	if err != nil || order == nil {
		failure(c, "订单不存在")
		return
	}
	secret := orderSecret(order)
	success(c, gin.H{"secret": secret, "widget": nil, "status": sharedOrderStatus(order, secret)})
}

func (h *Handler) tradeResponse(c *gin.Context, order *orderdomain.Order) {
	if order == nil {
		failure(c, "订单处理中，请使用相同request_no重试")
		return
	}
	secret := orderSecret(order)
	if secret == "" && !terminalFailure(order.Status) {
		failure(c, "订单处理中，请使用相同request_no重试")
		return
	}
	if terminalFailure(order.Status) {
		failure(c, "订单处理失败")
		return
	}
	success(c, gin.H{
		"url": "", "amount": order.TotalAmount.StringFixed(2),
		"tradeNo": strconv.FormatUint(uint64(order.ID), 10), "secret": secret,
	})
}

func (h *Handler) waitForOrder(userID, orderID uint, timeout time.Duration) *orderdomain.Order {
	deadline := time.Now().Add(timeout)
	for {
		order, err := h.Orders.GetOrderByUser(orderID, userID)
		if err == nil && order != nil && (orderSecret(order) != "" || terminalFailure(order.Status)) {
			return order
		}
		if time.Now().After(deadline) {
			return order
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func orderSecret(order *orderdomain.Order) string {
	if order == nil {
		return ""
	}
	fulfillment := order.Fulfillment
	if fulfillment == nil {
		for index := range order.Children {
			if order.Children[index].Fulfillment != nil {
				fulfillment = order.Children[index].Fulfillment
				break
			}
		}
	}
	if fulfillment == nil || fulfillment.Status != constants.FulfillmentStatusDelivered {
		return ""
	}
	if strings.TrimSpace(fulfillment.Payload) != "" {
		return fulfillment.Payload
	}
	if fulfillment.LogisticsJSON != nil {
		if cards, ok := fulfillment.LogisticsJSON["cards"]; ok {
			return stringifyCards(cards)
		}
	}
	return ""
}

func stringifyCards(value any) string {
	switch cards := value.(type) {
	case []string:
		return strings.Join(cards, "\n")
	case []any:
		rows := make([]string, 0, len(cards))
		for _, card := range cards {
			if row := strings.TrimSpace(fmt.Sprint(card)); row != "" {
				rows = append(rows, row)
			}
		}
		return strings.Join(rows, "\n")
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func sharedOrderStatus(order *orderdomain.Order, secret string) int {
	if secret != "" {
		return 1
	}
	if terminalFailure(order.Status) {
		return 2
	}
	return 0
}

func terminalFailure(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case constants.OrderStatusCanceled, constants.OrderStatusRefunded:
		return true
	default:
		return false
	}
}

func sharedOrderError(err error) string {
	switch {
	case errors.Is(err, upstreamhttp.ErrWalletInsufficient):
		return "余额不足"
	case errors.Is(err, upstreamhttp.ErrStockInsufficient):
		return "库存不足"
	case errors.Is(err, upstreamhttp.ErrProductUnavailable), errors.Is(err, upstreamhttp.ErrSKUUnavailable):
		return "商品不存在或已停售"
	default:
		return "创建订单失败"
	}
}
