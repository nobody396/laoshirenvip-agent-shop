# 验收环境状态

更新时间：2026-09-05 20:32（北京时间）

## 服务器

- Hostinger VM `1957721`，KVM 2，Ubuntu 24.04 LTS；
- 2 vCPU、8 GiB RAM、100 GB 磁盘；
- SSH 别名：`laoshirenvip-agent`（非 root 的 `deploy` 用户）；
- Docker、Docker Compose、UFW 已安装；
- UFW 只开放 SSH、80、443；
- `/opt/laoshirenvip-agent-shop` 归 `deploy` 用户；
- 购买周期至 2026-10-05。
- 已部署镜像 `sha-31340c2df8dd4a59f74dac2078c38e699830b78d`；
- PostgreSQL、Redis、应用、Nginx 均为运行状态；
- 临时总站：`http://agent.187-53-134-185.sslip.io`；
- 临时验收子站：`http://acceptance.shop.187-53-134-185.sslip.io`。

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
- 正式支付渠道、真实支付回调、支付手续费与退款手续费读回；
- 真实退款与人工提现打款；退款冲回已有自动化集成测试，但未制造真钱退款；
- 正式域名、wildcard DNS、TLS 与客户可见路径；
- 异地备份和隔离恢复演练。

以上属于生产资金或域名变更，不在验收环境中伪造。老板验收后再逐项开启。
