# 支付与上游采购链路留痕

这里的“录制”是接口链路留痕，不是屏幕录像。应用把一次真实订单的关键交互写入持久化结构化日志，便于核对发起参数、同步响应、异步回调、采购和交付状态。

## 一次成功订单的事件顺序

1. `integration_trace_payment_request`：本站准备发给支付网关的订单号、金额、币种、固定付款标题、回调地址和返回地址。
2. `integration_trace_payment_response`：支付网关同步受理结果、网关流水号及收银台地址摘要。
3. `epay_callback_received`：ZPay 异步回调字段；`sign` 只记录为 `[REDACTED]`。
4. `epay_callback_processed`：验签、商户归属、订单号、金额和状态检查全部通过，支付状态已提交。
5. `integration_trace_sharedstock_request`：本站发给 Aisou SharedStock 的采购参数。
6. `integration_trace_sharedstock_response`：Aisou HTTP 状态、业务响应、响应体大小与 SHA-256；卡密等交付内容只记录为 `[REDACTED]`。
7. `procurement_order_accepted`：采购单已被 Aisou 接受。
8. `procurement_order_fulfilled`：交付已落库，并触发客户交付及下游回调。

如果验签或金额检查失败，会出现 `epay_callback_handle_failed`，不会把订单错误标为已支付。若 HTTP 请求结果不确定，采购幂等键仍使用原订单号，不能换号重下。

## 导出一笔订单的记录

```bash
./scripts/export-integration-trace.sh <订单号或网关订单号>
```

导出文件默认位于：

```text
output/integration-traces/<订单号或网关订单号>.jsonl
```

应用日志保存在 Docker `logs` volume；容器重建不会清空。支付成功后的完整回调载荷还会保存到 `payments.provider_payload`，采购状态保存在 `procurement_orders`。

## 安全边界

- 不记录商户密钥、API Secret、签名原文、卡密、Token 或密码；
- 对原始响应保存大小和 SHA-256，既能证明本次响应内容没有被后改，也不会把交付凭证泄露到日志；
- CI 和模拟回调只能证明代码路径，最终验收仍必须以正式 HTTPS 域名上的真实支付回调和真实 Aisou 采购为准。
