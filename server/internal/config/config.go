package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Port           int      `yaml:"port"`
		JWT            string   `yaml:"jwt_secret"`
		CorsOrigins    []string `yaml:"cors_origins"`
		MQTTAuthSecret string   `yaml:"mqtt_auth_secret"` // EMQX 认证/ACL 回调共享密钥
	} `yaml:"server"`
	MySQL struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
	} `yaml:"mysql"`
	MQTT struct {
		Broker   string `yaml:"broker"`
		ClientID string `yaml:"client_id"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"mqtt"`
	Device struct {
		// OnlineMode 设备在线判定模式：
		//   connection — 按 MQTT 连接判定：connected=在线 / disconnected=离线，
		//                设备靠 MQTT keepalive 保活，不上报数据也在线；
		//                周期扫描仅作 broker 宕机兜底。
		//   report     — 按数据上报判定：超过 offline_timeout_sec 未上报判离线
		//                （设备保持连接也判离线）；offline_timeout_sec=0 禁用超时判定。
		//   ping       — 按应答信号判定：平台每 ping_interval_sec 向 wendao/{id}/ping 发探活，
		//                设备回 ping/ack 保活，超过 offline_timeout_sec 未应答判离线
		//                （设备保持连接也判离线）；适用于常供电常连接设备。
		// 三种模式下 MQTT 断开事件都立即置离线（连接即在线前提不被超时模式改变）。
		OnlineMode        string `yaml:"online_mode"`
		OfflineTimeoutSec int    `yaml:"offline_timeout_sec"` // report/ping 模式：超时秒数；connection 模式：兜底扫描阈值
		ScanIntervalSec   int    `yaml:"scan_interval_sec"`   // 离线检测扫描周期
		PingIntervalSec   int    `yaml:"ping_interval_sec"`   // ping 模式探活发送周期（秒）
	} `yaml:"device"`
	// EMQX REST API（互踢：同设备重复建连时踢掉同 username 的旧会话）。
	// APIKey/APISecret 为空时互踢功能静默禁用（仅打日志），不影响启动。
	EMQX struct {
		APIBase   string `yaml:"api_base"`   // 如 http://127.0.0.1:18083
		APIKey    string `yaml:"api_key"`    // EMQX Dashboard 创建的 API Key
		APISecret string `yaml:"api_secret"` // 与 API Key 配对的 Secret
	} `yaml:"emqx"`
}

func (c *Config) MySQLDSN() string {
	return c.MySQL.User + ":" + c.MySQL.Password +
		"@tcp(" + c.MySQL.Host + ":" + strconv.Itoa(c.MySQL.Port) + ")/" +
		c.MySQL.Database + "?charset=utf8mb4&parseTime=True&loc=Local"
}

// 已知的不安全默认密钥（历史遗留值），生产/常规启动一律拒绝。
var weakSecrets = map[string]bool{
	"wendaoiot-secret-key": true,
	"secret":               true,
	"changeme":             true,
	"admin123":             true,
}

func Load(path string) (*Config, error) {
	cfg := &Config{}
	// 配置文件可选：文件缺失时允许纯环境变量配置（12-factor），其它读取错误仍报错。
	if data, err := os.ReadFile(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		log.Printf("config: 配置文件 %s 不存在，改用环境变量/默认值", path)
	} else if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.applyEnv()

	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.MySQL.Port == 0 {
		cfg.MySQL.Port = 3306
	}
	if cfg.MQTT.ClientID == "" {
		cfg.MQTT.ClientID = "wendao_server"
	}
	// 设备在线判定模式：校验取值并归一化（applyEnv 已先执行，环境变量同样受此校验）
	switch strings.ToLower(strings.TrimSpace(cfg.Device.OnlineMode)) {
	case "", "connection":
		cfg.Device.OnlineMode = "connection"
	case "report":
		cfg.Device.OnlineMode = "report"
	case "ping":
		cfg.Device.OnlineMode = "ping"
	default:
		return nil, fmt.Errorf("device.online_mode 取值 %q 无效：仅支持 connection / report / ping", cfg.Device.OnlineMode)
	}
	// 离线超时默认值按模式区分：connection 给兜底阈值；report/ping 给 180s（建议 >=2 倍 ping 周期）
	switch cfg.Device.OnlineMode {
	case "connection":
		if cfg.Device.OfflineTimeoutSec <= 0 {
			cfg.Device.OfflineTimeoutSec = 60
		}
	default: // report / ping
		if cfg.Device.OfflineTimeoutSec <= 0 {
			cfg.Device.OfflineTimeoutSec = 180
		}
	}
	if cfg.Device.ScanIntervalSec <= 0 {
		cfg.Device.ScanIntervalSec = 60
	}
	if cfg.Device.PingIntervalSec <= 0 {
		cfg.Device.PingIntervalSec = 60
	}
	if cfg.Device.OnlineMode == "ping" && cfg.Device.OfflineTimeoutSec < cfg.Device.PingIntervalSec*2 {
		log.Printf("config: ping 模式 offline_timeout_sec(%d) 建议不小于 ping 周期(%d) 的 2 倍，否则可能误判离线",
			cfg.Device.OfflineTimeoutSec, cfg.Device.PingIntervalSec)
	}
	return cfg, nil
}

// insecure 为 true 时跳过密钥强度校验，仅供本地开发/测试。
func (c *Config) insecure() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("WQ_INSECURE"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Validate 校验启动期必须满足的安全配置。密钥缺失/为默认弱值/过短均拒绝启动。
func (c *Config) Validate() error {
	if c.insecure() {
		log.Println("config: WQ_INSECURE 已开启，跳过密钥强度校验（仅限本地开发，勿用于生产）")
		return nil
	}
	secret := c.Server.JWT
	switch {
	case secret == "":
		return errors.New("server.jwt_secret 未配置：请在 config.yaml 设置强随机密钥，或通过环境变量 WQ_JWT_SECRET 注入（本地临时测试可设 WQ_INSECURE=1）")
	case weakSecrets[secret]:
		return fmt.Errorf("server.jwt_secret 使用了不安全的默认值 %q，请更换为强随机密钥（或设置 WQ_INSECURE=1 仅供本地测试）", secret)
	case len(secret) < 16:
		return errors.New("server.jwt_secret 长度至少需要 16 位（或设置 WQ_INSECURE=1 仅供本地测试）")
	}
	return nil
}

// applyEnv 用 WQ_ 前缀环境变量覆盖配置文件值（环境变量优先级更高）。
func (c *Config) applyEnv() {
	if v := os.Getenv("WQ_SERVER_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Server.Port = n
		}
	}
	if v := os.Getenv("WQ_JWT_SECRET"); v != "" {
		c.Server.JWT = v
	}
	if v := os.Getenv("WQ_MQTT_AUTH_SECRET"); v != "" {
		c.Server.MQTTAuthSecret = v
	}
	c.Server.CorsOrigins = appendEnvv(c.Server.CorsOrigins, os.Getenv("WQ_CORS_ORIGINS"))

	if v := os.Getenv("WQ_MYSQL_HOST"); v != "" {
		c.MySQL.Host = v
	}
	if v := os.Getenv("WQ_MYSQL_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.MySQL.Port = n
		}
	}
	if v := os.Getenv("WQ_MYSQL_USER"); v != "" {
		c.MySQL.User = v
	}
	if v := os.Getenv("WQ_MYSQL_PASSWORD"); v != "" {
		c.MySQL.Password = v
	}
	if v := os.Getenv("WQ_MYSQL_DATABASE"); v != "" {
		c.MySQL.Database = v
	}

	if v := os.Getenv("WQ_MQTT_BROKER"); v != "" {
		c.MQTT.Broker = v
	}
	if v := os.Getenv("WQ_MQTT_CLIENT_ID"); v != "" {
		c.MQTT.ClientID = v
	}
	if v := os.Getenv("WQ_MQTT_USERNAME"); v != "" {
		c.MQTT.Username = v
	}
	if v := os.Getenv("WQ_MQTT_PASSWORD"); v != "" {
		c.MQTT.Password = v
	}

	if v := os.Getenv("WQ_DEVICE_ONLINE_MODE"); v != "" {
		c.Device.OnlineMode = v
	}
	if v := os.Getenv("WQ_DEVICE_OFFLINE_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Device.OfflineTimeoutSec = n
		}
	}
	if v := os.Getenv("WQ_DEVICE_SCAN_INTERVAL_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Device.ScanIntervalSec = n
		}
	}
	if v := os.Getenv("WQ_DEVICE_PING_INTERVAL_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Device.PingIntervalSec = n
		}
	}

	if v := os.Getenv("WQ_EMQX_API_BASE"); v != "" {
		c.EMQX.APIBase = v
	}
	if v := os.Getenv("WQ_EMQX_API_KEY"); v != "" {
		c.EMQX.APIKey = v
	}
	if v := os.Getenv("WQ_EMQX_API_SECRET"); v != "" {
		c.EMQX.APISecret = v
	}
}

// appendEnvv 把逗号分隔的环境变量值追加到切片（用于 CORS 白名单）。
func appendEnvv(base []string, v string) []string {
	if v == "" {
		return base
	}
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			base = append(base, p)
		}
	}
	return base
}

// ConfigPath 返回配置文件路径，可由 WQ_CONFIG_PATH 覆盖。
func ConfigPath() string {
	if v := os.Getenv("WQ_CONFIG_PATH"); v != "" {
		return v
	}
	return "config.yaml"
}
