# 验收环境备份与恢复

服务器每天执行 `/opt/laoshirenvip-agent-shop/bin/backup.sh`，备份 PostgreSQL 与上传文件，默认保留 7 天。当前是同机备份，只能防误操作，不能替代异地备份。

## 手动备份

```bash
sudo /opt/laoshirenvip-agent-shop/bin/backup.sh
```

每个备份目录包含：

- `postgres.dump`：PostgreSQL custom format；
- `uploads.tar.gz`：上传文件；
- `SHA256SUMS`：完整性校验。

## 恢复演练

恢复前先停止应用和 worker，禁止在写入中的生产库上直接覆盖：

```bash
cd /opt/laoshirenvip-agent-shop/deploy
docker stop deploy-app-1
sha256sum -c /opt/laoshirenvip-agent-shop/backups/TIMESTAMP/SHA256SUMS
docker exec -i deploy-postgres-1 pg_restore \
  -U agent_shop -d agent_shop --clean --if-exists \
  < /opt/laoshirenvip-agent-shop/backups/TIMESTAMP/postgres.dump
docker run --rm \
  -v deploy_uploads:/data \
  -v /opt/laoshirenvip-agent-shop/backups/TIMESTAMP:/backup:ro \
  alpine:3.21 sh -c 'cd /data && tar -xzf /backup/uploads.tar.gz'
docker start deploy-app-1
```

恢复后依次检查 `/health`、总站商品数、测试子站隔离和上游只读连接。正式上线前还需要将加密备份同步到另一台机器或对象存储，并完成一次隔离恢复演练。
