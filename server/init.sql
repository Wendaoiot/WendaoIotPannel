-- ============================================================================
-- init.sql —— 数据库初始化入口（编码安全版）
--
-- 设计要点（为什么这样写）：
--   1. 中文种子数据一律用 0xXXXX 十六进制 UTF-8 字面量 + CONVERT(... USING utf8mb4)
--      表达。原因：.sql 文件在 Windows 上常被 mysql 客户端按 cp936/latin1 误读
--      （默认字符集跟随系统码页），文件里的中文字符串会被静默转成 '?' 坏字节。
--      hex 字面量在任何码页下字节都不变，彻底免疫编码问题。
--   2. 表结构由 GORM AutoMigrate 自动创建（server 启动时执行），本文件不建表，
--      避免与 GORM 的 schema 演进产生两套真相。
--   3. 业务数据（租户/项目/设备）的种子全部走 INSERT ... SELECT ... WHERE NOT EXISTS
--      幂等写法，可重复执行。
--
-- 执行方式（关键：显式指定 utf8mb4，勿依赖系统码页）：
--   mysql -uroot -p --default-character-set=utf8mb4 < server/init.sql
--   或容器内：
--   docker exec -i wq-mysql mysql -uroot -pXXX --default-character-set=utf8mb4 wendaoiot < server/init.sql
-- ============================================================================

-- ---------------------------------------------------------------------------
-- 1. 数据库与默认字符集
-- ---------------------------------------------------------------------------
CREATE DATABASE IF NOT EXISTS wendaoiot
  DEFAULT CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

USE wendaoiot;

-- 会话级兜底：即使客户端连接参数缺失，本会话内写入也按 utf8mb4 处理
SET NAMES utf8mb4;

-- ---------------------------------------------------------------------------
-- 2. 表结构
--    表由后端 GORM AutoMigrate 维护；此处仅提示执行顺序：
--    先跑 init.sql 建库 -> 再启动 server（自动建表+种子管理员）。
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- 3. 可选：演示种子数据（幂等，重复执行无副作用）
--    名称以 UTF-8 hex 字面量书写：
--      0xE88194E8B083E6B58BE8AF95E585ACE58FB8 = '联调测试公司'
--      0xE88194E8B083E9A1B9E79BAE             = '联调项目'
--      0xE88194E8B083E8AEBEE5A48741           = '联调设备A'
--      0xE88194E8B083E8AEBEE5A48742           = '联调设备B'
--    注意：设备接入密钥(device_secret, bcrypt)由平台 API 生成才能与 EMQX
--    认证回调配套；SQL 只建「骨架」，创建后在管理后台「密钥」按钮重置获取，
--    或用 seed_devices.ps1（已修复编码问题）走 API 创建。
-- ---------------------------------------------------------------------------

-- 3.1 租户
INSERT INTO tenants (name, created_at, updated_at)
SELECT CONVERT(0xE88194E8B083E6B58BE8AF95E585ACE58FB8 USING utf8mb4), NOW(), NOW()
WHERE NOT EXISTS (
  SELECT 1 FROM tenants
  WHERE name = CONVERT(0xE88194E8B083E6B58BE8AF95E585ACE58FB8 USING utf8mb4)
);

-- 3.2 项目（归属上面的租户）
INSERT INTO projects (tenant_id, name, created_at, updated_at)
SELECT t.id,
       CONVERT(0xE88194E8B083E9A1B9E79BAE USING utf8mb4), NOW(), NOW()
FROM tenants t
WHERE t.name = CONVERT(0xE88194E8B083E6B58BE8AF95E585ACE58FB8 USING utf8mb4)
  AND NOT EXISTS (
    SELECT 1 FROM projects p
    WHERE p.tenant_id = t.id
      AND p.name = CONVERT(0xE88194E8B083E9A1B9E79BAE USING utf8mb4)
  );

-- 3.3 设备骨架（无密钥；密钥创建后在管理后台「密钥」按钮重置获取）
INSERT INTO devices (id, project_id, tenant_id, name, status, enabled, first_ts, created_at, updated_at)
SELECT 'DEVA001', p.id, p.tenant_id,
       CONVERT(0xE88194E8B083E8AEBEE5A48741 USING utf8mb4), 0, 0, 0, NOW(), NOW()
FROM projects p
WHERE p.name = CONVERT(0xE88194E8B083E9A1B9E79BAE USING utf8mb4)
  AND NOT EXISTS (
    SELECT 1 FROM devices d WHERE d.id = 'DEVA001'
  );

INSERT INTO devices (id, project_id, tenant_id, name, status, enabled, first_ts, created_at, updated_at)
SELECT 'DEVB001', p.id, p.tenant_id,
       CONVERT(0xE88194E8B083E8AEBEE5A48742 USING utf8mb4), 0, 0, 0, NOW(), NOW()
FROM projects p
WHERE p.name = CONVERT(0xE88194E8B083E9A1B9E79BAE USING utf8mb4)
  AND NOT EXISTS (
    SELECT 1 FROM devices d WHERE d.id = 'DEVB001'
  );

-- ---------------------------------------------------------------------------
-- 验证（可选，手动执行）：
--   SELECT id, name, HEX(name) FROM devices;   -- hex 应为 E88194... 开头
--   SELECT id, name, HEX(name) FROM projects;
--   SELECT id, name, HEX(name) FROM tenants;
--   若 HEX(name) 出现 3F3F3F...（即 '?'），说明执行时未带
--   --default-character-set=utf8mb4，请检查执行命令。
-- ---------------------------------------------------------------------------
