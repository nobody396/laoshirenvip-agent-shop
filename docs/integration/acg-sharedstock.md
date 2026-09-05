# 异次元 ACG SharedStock 下游对接

本站同时提供异次元 ACG 的新版 SharedStock 与旧版 SharedStock 插件接口。两种接口共用同一代理账户、采购余额、订单和交付记录。

异次元下游在“共享店铺”中配置本站。正式兼容面同时覆盖：

- core：`/shared/authentication/*`、`/shared/commodity/*`；
- legacy：`/plugin/SharedStock/api/*`。

## 开通

1. 在代理总站注册账户并充值采购余额。
2. 在个人中心申请 API 凭证，等待管理员审核。
3. 审核通过后保存只展示一次的 API Secret。
4. 在异次元后台新增“店铺共享”：

| 字段 | 值 |
|---|---|
| 店铺地址 | 代理总站正式 HTTPS 根地址，不要附加接口路径 |
| 类型 | 优先选新版/SharedStock；旧站可选旧版 SharedStock 插件 |
| 商户 ID / App ID | 本站 API Key |
| 商户密钥 / App Key | 本站 API Secret |

保存后先运行连接测试，再同步商品。本站支持商品列表、单品详情、库存检测、估价、余额下单、幂等重试和订单查询。

## 接口

| 能力 | 新版路径 | 旧版路径 |
|---|---|---|
| 连接 | `/shared/authentication/connect` | `/plugin/SharedStock/api/connect` |
| 商品列表 | `/shared/commodity/items` | `/plugin/SharedStock/api/items` |
| 商品详情 | `/shared/commodity/item` | `/plugin/SharedStock/api/item` |
| 库存 | `/shared/commodity/inventory` | `/plugin/SharedStock/api/inventory` |
| 库存检查 | `/shared/commodity/inventoryState` | `/plugin/SharedStock/api/inventoryState` |
| 估价 | `/shared/commodity/valuation` | `/plugin/SharedStock/api/valuation` |
| 下单 | `/shared/commodity/trade` | `/plugin/SharedStock/api/trade` |
| 查询 | `/shared/commodity/query` | `/plugin/SharedStock/api/query` |

下游使用管理员批准的商户 ID 与 App Key。商品、库存、报价、交易和查询全部映射到同一份 Dujiao 商品、钱包、订单和交付账本，不建立第二套资金状态机。

安全规则：

- 所有表单按 ACG SharedValidation 规则签名；
- 商户 ID 只能访问自己的钱包和订单；
- `request_no` 永久幂等；
- 不确定采购不得以新编号重试；
- 响应只返回下游所需交付内容，不暴露更上游凭证或内部诊断。

SharedStock 要求同步下单尽快返回卡密。本站会在余额支付后等待最多 25 秒；若仍显示“订单处理中”，必须保持原 `request_no` 重试或先查询，禁止换编号重新采购。
