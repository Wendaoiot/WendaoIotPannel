package config

import "testing"

// 不存在的配置文件路径：用于验证“纯环境变量”启动方式。
const missingPath = "definitely-missing-config.yaml"

func load(t *testing.T) *Config {
	t.Helper()
	cfg, err := Load(missingPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestValidate_RejectsWeakSecret(t *testing.T) {
	t.Setenv("WQ_INSECURE", "")
	t.Setenv("WQ_JWT_SECRET", "wendaoiot-secret-key")
	cfg := load(t)
	if err := cfg.Validate(); err == nil {
		t.Fatal("弱默认密钥应被拒绝启动")
	}
}

func TestValidate_RejectsEmptyAndShortSecret(t *testing.T) {
	t.Setenv("WQ_INSECURE", "")
	t.Setenv("WQ_JWT_SECRET", "")
	if err := load(t).Validate(); err == nil {
		t.Fatal("空密钥应被拒绝启动")
	}

	t.Setenv("WQ_JWT_SECRET", "short")
	if err := load(t).Validate(); err == nil {
		t.Fatal("过短密钥(<16)应被拒绝启动")
	}
}

func TestValidate_AcceptsStrongSecret(t *testing.T) {
	t.Setenv("WQ_INSECURE", "")
	t.Setenv("WQ_JWT_SECRET", "this-is-a-very-long-random-secret-0123456789")
	if err := load(t).Validate(); err != nil {
		t.Fatalf("强密钥应通过校验: %v", err)
	}
}

func TestValidate_InsecureBypass(t *testing.T) {
	t.Setenv("WQ_INSECURE", "1")
	t.Setenv("WQ_JWT_SECRET", "wendaoiot-secret-key")
	if err := load(t).Validate(); err != nil {
		t.Fatalf("WQ_INSECURE=1 时应跳过校验: %v", err)
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("WQ_INSECURE", "1")
	t.Setenv("WQ_JWT_SECRET", "env-secret-value-1234567890")
	t.Setenv("WQ_SERVER_PORT", "9999")
	t.Setenv("WQ_MYSQL_HOST", "db.example.com")
	t.Setenv("WQ_MYSQL_DATABASE", "iotdb")
	t.Setenv("WQ_MQTT_BROKER", "tls://broker.example.com:8883")

	cfg := load(t)

	if cfg.Server.JWT != "env-secret-value-1234567890" {
		t.Errorf("WQ_JWT_SECRET 未生效, got %q", cfg.Server.JWT)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("WQ_SERVER_PORT 未生效, got %d", cfg.Server.Port)
	}
	if cfg.MySQL.Host != "db.example.com" || cfg.MySQL.Database != "iotdb" {
		t.Errorf("WQ_MYSQL_* 未生效, got %+v", cfg.MySQL)
	}
	if cfg.MQTT.Broker != "tls://broker.example.com:8883" {
		t.Errorf("WQ_MQTT_BROKER 未生效, got %q", cfg.MQTT.Broker)
	}
}

func TestDefaults(t *testing.T) {
	t.Setenv("WQ_INSECURE", "1")
	cfg := load(t)
	if cfg.Server.Port != 8080 {
		t.Errorf("默认端口应为 8080, got %d", cfg.Server.Port)
	}
	if cfg.MySQL.Port != 3306 {
		t.Errorf("MySQL 默认端口应为 3306, got %d", cfg.MySQL.Port)
	}
	if cfg.MQTT.ClientID != "wendao_server" {
		t.Errorf("MQTT 默认 client_id 异常, got %q", cfg.MQTT.ClientID)
	}
}

func TestMySQLDSN(t *testing.T) {
	cfg := load(t)
	cfg.MySQL.User = "u"
	cfg.MySQL.Password = "p"
	cfg.MySQL.Host = "h"
	cfg.MySQL.Port = 3306
	cfg.MySQL.Database = "d"
	dsn := cfg.MySQLDSN()
	want := "u:p@tcp(h:3306)/d?charset=utf8mb4&parseTime=True&loc=Local"
	if dsn != want {
		t.Errorf("DSN mismatch:\n got %s\nwant %s", dsn, want)
	}
}
