// Package broker 将 mochi-mqtt（MIT 协议）内嵌进 wendao-server 进程，替代外部 EMQX。
//
// 职责：
//   - 装配 MQTT TCP/TLS listener（1883 / 8883）；
//   - 注册 WendaoHook 完成进程内认证、ACL、设备上下线同步；
//   - 维护 sessionIndex（username→会话）实现同设备重复连接互踢；
//   - 可选 pebble 持久化（离线 QoS1 消息在 clean-session=false 设备重连后补收）。
//
// 服务端自身业务收发不走内嵌 API，而是 paho 客户端回环自连 127.0.0.1:{listen_port}
// （凭据为 broker.server_username/password，认证时按"平台账号"分支放行），
// 因此 internal/mqtt 的全部业务处理逻辑保持零改动。
package broker

import (
	"crypto/tls"
	"fmt"
	"log"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/storage/pebble"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"

	"wendaoiotpannel/internal/events"
	"wendaoiotpannel/internal/model"
)

// Config 为 broker 装配参数（来源 config.Broker，见 config.Load 的默认值与环境变量）。
type Config struct {
	ListenPort      int    // MQTT TCP 端口（1883）
	TLSPort         int    // MQTT over TLS 端口（0=禁用）
	TLSCert         string // TLS 证书路径（TLSPort>0 时必填）
	TLSKey          string // TLS 私钥路径（TLSPort>0 时必填）
	PersistencePath string // pebble 持久化目录（空=禁用持久化）
	ServerUsername  string // 平台自身 MQTT 账号（paho 回环连接使用）
	ServerPassword  string // 平台自身 MQTT 密码
}

// AuthSource 认证/上下线所需的最小存储接口（*store.Store 天然满足；
// 抽象为接口以便单测注入假实现）。
type AuthSource interface {
	GetDeviceAuthRecord(deviceID string) (*model.Device, error)
	GetProductByKey(productKey string) (*model.Product, error)
	GetDevice(deviceID string) (*model.Device, error)
	SetDeviceRuntimeStatus(deviceID string, online bool) error
}

// Broker 内嵌 MQTT broker。
type Broker struct {
	srv      *mqtt.Server
	cfg      Config
	store    AuthSource
	bus      *events.Bus
	sessions *sessionIndex
}

// New 创建并完成 hook 注册（不监听端口，调用 Start 开始服务）。
func New(cfg Config, s AuthSource, bus *events.Bus) (*Broker, error) {
	b := &Broker{
		cfg:      cfg,
		store:    s,
		bus:      bus,
		sessions: newSessionIndex(),
	}
	b.srv = mqtt.New(&mqtt.Options{
		// inline client：允许 server.Publish 从进程内直接向任意主题投递消息
		// （互踢 kicked 通知、以及将来可能的运维通道都依赖它）。
		InlineClient: true,
	})

	// 认证 / ACL / 上下线 hook（必须最先注册）
	if err := b.srv.AddHook(&WendaoHook{broker: b}, nil); err != nil {
		return nil, fmt.Errorf("add wendao hook: %w", err)
	}

	// 可选持久化：会话/订阅/inflight/retained 落 pebble，进程重启后 clean-session=false
	// 的设备可补收离线期间的 QoS1 消息（对齐原 EMQX 会话队列语义）。
	if cfg.PersistencePath != "" {
		if err := b.srv.AddHook(new(pebble.Hook), &pebble.Options{
			Path: cfg.PersistencePath,
			Mode: pebble.NoSync,
		}); err != nil {
			return nil, fmt.Errorf("add pebble storage hook: %w", err)
		}
	}
	return b, nil
}

// Start 装配 listener 并开始接受连接（非阻塞，Serve 在后台 goroutine 运行）。
func (b *Broker) Start() error {
	if b.cfg.ListenPort > 0 {
		if err := b.srv.AddListener(listeners.NewTCP(listeners.Config{
			ID:      fmt.Sprintf("tcp%d", b.cfg.ListenPort),
			Address: fmt.Sprintf(":%d", b.cfg.ListenPort),
		})); err != nil {
			return fmt.Errorf("add tcp listener: %w", err)
		}
	}

	if b.cfg.TLSPort > 0 {
		cert, err := tls.LoadX509KeyPair(b.cfg.TLSCert, b.cfg.TLSKey)
		if err != nil {
			return fmt.Errorf("load tls cert: %w", err)
		}
		if err := b.srv.AddListener(listeners.NewTCP(listeners.Config{
			ID:        fmt.Sprintf("tls%d", b.cfg.TLSPort),
			Address:   fmt.Sprintf(":%d", b.cfg.TLSPort),
			TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		})); err != nil {
			return fmt.Errorf("add tls listener: %w", err)
		}
	}

	go func() {
		if err := b.srv.Serve(); err != nil {
			log.Printf("broker serve exited: %v", err)
		}
	}()
	log.Printf("broker started: tcp=%d tls=%d persistence=%q server_user=%s",
		b.cfg.ListenPort, b.cfg.TLSPort, b.cfg.PersistencePath, b.cfg.ServerUsername)
	return nil
}

// Close 停止 broker 并断开全部客户端（main 退出时调用）。
func (b *Broker) Close() error {
	log.Println("broker closing")
	return b.srv.Close()
}

// KickByUsername 踢掉某 username（设备ID / 引导用户名）名下的全部会话，
// excludeClientID 用于互踢时排除新连接自身；传空踢全部。
// 返回成功断开的 clientID 列表。
func (b *Broker) KickByUsername(username, excludeClientID string) ([]string, error) {
	if username == "" {
		return nil, nil
	}
	var kicked []string
	for _, cl := range b.sessions.List(username) {
		if cl.ID == excludeClientID {
			continue
		}
		// MQTT5 向客户端发 DISCONNECT(0x98 administrative action) 后断开；v3 直接断开。
		if err := b.srv.DisconnectClient(cl, packets.ErrAdministrativeAction); err != nil {
			log.Printf("broker kick %s (client %s) error: %v", username, cl.ID, err)
			continue
		}
		kicked = append(kicked, cl.ID)
	}
	return kicked, nil
}

// KickDeviceSessions 满足 handler.SessionKicker 接口（互踢入口），
// 语义与 KickByUsername 一致：设备ID 即 MQTT username。
func (b *Broker) KickDeviceSessions(deviceID, excludeClientID string) ([]string, error) {
	return b.KickByUsername(deviceID, excludeClientID)
}
