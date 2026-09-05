# 验收环境状态

更新时间：2026-09-05 18:27（北京时间）

## 服务器

- Hostinger VM `1957721`，KVM 2，Ubuntu 24.04 LTS；
- 2 vCPU、8 GiB RAM、100 GB 磁盘；
- SSH 别名：`laoshirenvip-agent`（非 root 的 `deploy` 用户）；
- Docker、Docker Compose、UFW 已安装；
- UFW 只开放 SSH、80、443；
- `/opt/laoshirenvip-agent-shop` 归 `deploy` 用户；
- 购买周期至 2026-10-05。

## 只读上游验证

使用 Agent Switch 中的 `AISOU_MERCHANT_ID` 与 `AISOU_MERCHANT_KEY`，通过本项目 SharedStock 客户端执行了只读连接和目录探测：

- 站点识别为 Aisou 智充；
- 返回 8 个分类、20 个商品；
- Dujiao SharedStock adapter 同样返回 20 个商品，其中 1 个多 SKU 商品；
- 未调用交易接口，未扣 Aisou 余额，未创建真实订单。

## 尚未证明

- Aisou 真实采购、未知结果对账和交付回传；
- 下游异次元 SharedStock 兼容；
- 真钱支付、退款和提现；
- 正式域名、TLS 与客户可见路径。
