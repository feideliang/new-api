# Logs 表迁移方案

## 背景

线上 `logs` 表 **27GB**，上游 `v1.0.0-rc.5` 对 `Log` struct 做了以下变更：

1. 新增字段 `upstream_request_id VARCHAR(128) DEFAULT ''`
2. 索引优先级调整：`idx_created_at_id`、`idx_user_id_id`、`index_username_model_name`

直接 AutoMigrate 会在 27GB 表上执行 `ALTER TABLE` + **重建索引**，可能锁表数小时甚至卡死。

## 迁移策略

> 阈值：logs 表行数 > 100 万行时触发归档迁移，否则正常 AutoMigrate

### 步骤 1：备份（归档旧表）

```sql
RENAME TABLE logs TO logs_archive_20260612;
```

- 瞬间完成，不锁表
- 旧数据完整保留，可随时回滚

### 步骤 2：新建表

```sql
-- AutoMigrate 在空表上创建，毫秒级完成
-- GORM 会自动创建新字段和索引
```

新表结构包含 `upstream_request_id` 字段和调整后的索引。

### 步骤 3：迁入近 3 个月数据

```sql
INSERT INTO logs (
    user_id, created_at, type, content, username, token_name,
    model_name, quota, prompt_tokens, completion_tokens, use_time,
    is_stream, channel_id, token_id, `group`, ip, request_id,
    upstream_request_id, other
)
SELECT
    user_id, created_at, type, content, username, token_name,
    model_name, quota, prompt_tokens, completion_tokens, use_time,
    is_stream, channel_id, token_id, `group`, ip, request_id,
    '' AS upstream_request_id, other
FROM logs_archive_20260612
WHERE created_at >= UNIX_TIMESTAMP(DATE_SUB(NOW(), INTERVAL 3 MONTH));
```

- 只迁移近 3 个月数据，大幅减少迁移量
- 预计耗时取决于 3 个月数据量（远小于 27GB）

### 步骤 4：验证

```sql
-- 检查新表数据量
SELECT COUNT(*) FROM logs;

-- 对比归档表近 3 个月数据量
SELECT COUNT(*) FROM logs_archive_20260612
WHERE created_at >= UNIX_TIMESTAMP(DATE_SUB(NOW(), INTERVAL 3 MONTH));

-- 检查新表索引
SHOW INDEX FROM logs;
```

### 步骤 5：清理（确认无误后）

```sql
-- 确认业务正常后再执行
DROP TABLE logs_archive_20260612;
```

## 回滚方案

如果迁移后发现问题：

```sql
-- 删除新表
DROP TABLE logs;

-- 恢复旧表
RENAME TABLE logs_archive_20260612 TO logs;
```

## 注意事项

- **执行时间**：建议在低峰期操作（凌晨）
- **停止服务**：迁移期间需停止 new-api 服务，避免写入丢失
- **磁盘空间**：归档 + 新表需要额外 ~27GB 空间，迁移完成后可释放
- **PostgreSQL**：语法略有不同，`RENAME TABLE` 改为 `ALTER TABLE logs RENAME TO logs_archive_20260612`
