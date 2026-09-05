# 验收环境状态

更新时间：2026-09-05 23:20（北京时间）

## 服务器

- Hostinger VM `1957721`，KVM 2，Ubuntu 24.04 LTS；
- 2 vCPU、8 GiB RAM、100 GB 磁盘；
- SSH 别名：`laoshirenvip-agent`（非 root 的 `deploy` 用户）；
- Docker、Docker Compose、UFW 已安装；
- UFW 只开放 SSH、80、443；
- `/opt/laoshirenvip-agent-shop` 归 `deploy` 用户；
- 购买周期至 2026-10-05。
- 已部署镜像 `sha-d80f3aa19be3e8ed45515317abbaea192ec679ab`；
- PostgreSQL、Redis、应用、Nginx 均为运行状态；
- 临时总站：`http://agent.187-53-134-185.sslip.io`；
- 原临时验收子站已迁移为正式域名 `acceptance.lsrai.shop`；旧 sslip.io 子站地址不再作为租户入口。

## 正式域名与支付准备

- 正式总站：`https://lsrai.shop`；
- 正式验收子站：`https://acceptance.lsrai.shop`；
- 腾讯云 EdgeOne 个人版已购买 1 个月，自动续费关闭；站点使用全球可用区（不含中国大陆）和 NS 接入；
- Hostinger 域名服务器已提交切换至 `ns1.qeodns.com`、`ns2.qeodns.com`；切换期间旧 Hostinger DNS 和新 EdgeOne DNS 都指向 EdgeOne，避免解析空窗；
- 根域名 EdgeOne 免费证书已部署；`*.lsrai.shop` 泛域名免费证书仍处于平台自动申请阶段，证书部署前不得开始子站真钱验收；
- 源站 `80/443` 已启用，HTTPS 回源使用覆盖 `lsrai.shop` 与 `*.lsrai.shop` 的源站证书，EdgeOne 回源证书校验保持关闭；
- 动态 `/api/*`、`/shared/*`、`/plugin/SharedStock/*` 与 `/health` 均由应用返回 `Cache-Control: no-store`；指纹静态资源保持一年 immutable 缓存；
- ZPay 支付宝渠道 ID `1` 已启用，网关为 `https://zpayz.cn`，回调为 `https://lsrai.shop/api/v1/payments/callback`，费率 `1.60%` 且客户承担手续费；
- ZPay 余额查询只读验证返回成功，未创建测试支付单；
- 总站与默认子站公开配置均只显示支付宝；子站支付渠道列表为空时按产品约定自动回落为“仅支付宝”。

## 只读上游验证

使用 Agent Switch 中的 `AISOU_MERCHANT_ID` 与 `AISOU_MERCHANT_KEY`，通过本项目 SharedStock 客户端执行了只读连接和目录探测：

- 站点识别为 Aisou 智充；
- 返回 8 个分类、20 个商品；
- Dujiao SharedStock adapter 同样返回 20 个商品，其中 1 个多 SKU 商品；
- 未调用交易接口，未扣 Aisou 余额，未创建真实订单。

## 已完成验收

- 已导入 12 个商品、14 个启用 SKU，代理供货价与定价表一致；
- ACG SharedStock 新版与旧版的连接、目录、单品、库存、估价、合成余额下单和相同 `request_no` 幂等通过；
- Dujiao OpenAPI 的连接、合成余额下单、交付查询和下游订单号幂等通过；
- 测试子站独立品牌、10% 默认加价、未知子域名 404 隔离通过；
- 子站合成余额订单按 `¥10.00 → ¥11.00` 保存快照，生成 `¥1.00` 待确认利润；
- 到期任务将利润转为可用，测试提现 `¥0.50` 成功锁定为待审核；没有标记为已打款；
- 客户手续费附加策略已启用，未配置任何真钱支付渠道；
- 本地全量 `go test ./...` 通过；GitHub CI 的 API、fullstack、installer、release config 全部通过；
- GHCR linux/amd64 镜像构建成功并按不可变 SHA 部署。
- 每日同机备份任务已启用，首次 PostgreSQL/上传备份校验和通过；异地恢复仍是生产门槛。
- 浏览器实测总站标题、子站独立品牌和子站加价正确，客户页面不再显示上游项目品牌链接。
- 12 个商品已改用老实人 VIP GMShop 原有的 5 张分类封面；Aisou 商品长文案、外链和售后措辞已全部替换为老实人VIP统一详情模板。
- 顶部“博客”已替换为“对接教程”，公开页面覆盖独角兽 Dujiao OpenAPI、异次元 ACG SharedStock、新旧路径、签名、幂等和可直接复制给 AI 的完整提示词。

## 生产开放前仍需老板验收/授权

- Aisou 最小真实采购、未知结果对账和真实交付回传；
- 真实支付回调、支付手续费与退款手续费读回；
- 真实退款与人工提现打款；退款冲回已有自动化集成测试，但未制造真钱退款；
- `*.lsrai.shop` 泛域名证书从“申请中”变为“已部署”后的客户可见路径复核；
- 异地备份和隔离恢复演练。

以上属于生产资金或域名变更，不在验收环境中伪造。老板验收后再逐项开启。
