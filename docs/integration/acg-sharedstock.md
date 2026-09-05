# 异次元 ACG SharedStock 下游对接

> 当前为兼容契约草案；只有 SharedStock 服务端端到端测试通过后才能对外发布。

异次元下游在“共享店铺”中配置本站。正式兼容面同时覆盖：

- core：`/shared/authentication/*`、`/shared/commodity/*`；
- legacy：`/plugin/SharedStock/api/*`。

下游使用管理员批准的商户 ID 与 App Key。商品、库存、报价、交易和查询全部映射到同一份 Dujiao 商品、钱包、订单和交付账本，不建立第二套资金状态机。

安全规则：

- 所有表单按 ACG SharedValidation 规则签名；
- 商户 ID 只能访问自己的钱包和订单；
- `request_no` 永久幂等；
- 不确定采购不得以新编号重试；
- 响应只返回下游所需交付内容，不暴露更上游凭证或内部诊断。
