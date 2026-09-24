package broker

import (
	"bytes"
	"log"
	"strings"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"

	cryptopkg "wendaoiotpannel/pkg/crypto"

	"wendaoiotpannel/internal/protocol"
)

// bootstrapSep 与 protocol.BootstrapSep 一致（"SN&ProductKey" 引导用户名分隔符）。
// 直接引用 protocol 包会与 broker→store 的依赖方向交叉，这里用常量并在测试中校验一致性。
const bootstrapSep = "&"

// kickNoticeGrace 通知已入队到执行踢线之间预留的投递宽限：让 broker 有时间把
// QoS1 kicked 消息下发给被踢的在线会话（clean session=true 时消息不持久化，
// 此宽限是其收到原因的唯一窗口）。与 handler/mqtt_hook.go 原值一致。
var kickNoticeGrace = 600 * time.Millisecond

// WendaoHook 进程内实现原 EMQX 的四类集成点：
// 认证（OnConnectAuthenticate）、ACL（OnACLCheck）、设备上下线
// （OnSessionEstablished/OnDisconnect）、会话索引维护（供互踢）。
type WendaoHook struct {
	mqtt.HookBase
	broker *Broker
}

// ID 返回 hook 标识（日志中出现）。
func (h *WendaoHook) ID() string {
	return "wendao-auth-acl"
}

// Provides 声明本 hook 实现的回调（未声明的走 HookBase 默认空实现）。
func (h *WendaoHook) Provides(b byte) bool {
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
		mqtt.OnSessionEstablished,
		mqtt.OnDisconnect,
	}, []byte{b})
}

// OnConnectAuthenticate 设备接入认证（CONNECT 阶段，返回 bool）。
// 三分支语义与原 handler.EMQXAuthenticate 完全一致：
//  1. 平台自身账号（paho 回环连接）；
//  2. 一型一密引导连接（username="SN&ProductKey"，product_secret 校验+待激活）；
//  3. 普通设备（一机一密，bcrypt）。
//
// 互踢：普通设备凭据校验通过后，异步通知+踢掉同 username 的旧会话（排除本次连接），
// 不阻塞认证返回——与原"异步踢线避免 EMQX 认证超时"的设计相同。
func (h *WendaoHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	username := string(pk.Connect.Username)
	password := string(pk.Connect.Password)
	if username == "" {
		log.Printf("broker auth: deny (anonymous) clientid=%s", pk.Connect.ClientIdentifier)
		return false
	}

	// 分支1：平台自身账号（服务端 paho 回环连接）
	if username == h.broker.cfg.ServerUsername {
		if password == h.broker.cfg.ServerPassword {
			return true
		}
		log.Printf("broker auth: deny (server account bad password) clientid=%s", pk.Connect.ClientIdentifier)
		return false
	}

	// 分支2：一型一密引导连接
	if sn, productKey, isBootstrap := protocol.ParseBootstrapUsername(username); isBootstrap {
		if h.authBootstrap(sn, productKey, password) {
			return true
		}
		return false
	}

	// 分支3：普通设备（一机一密）
	dev, err := h.broker.store.GetDeviceAuthRecord(username)
	if err != nil {
		log.Printf("broker auth: deny (device not found) user=%s", username)
		return false
	}
	if !dev.Enabled {
		log.Printf("broker auth: deny (device disabled) user=%s", username)
		return false
	}
	if dev.DeviceSecret == "" {
		log.Printf("broker auth: deny (secret not provisioned) user=%s", username)
		return false
	}
	if !cryptopkg.CheckPassword(dev.DeviceSecret, password) {
		log.Printf("broker auth: deny (bad credentials) user=%s", username)
		return false
	}

	// 认证通过 → 异步互踢（通知旧会话 + 排除本次新连接），不阻塞认证响应。
	msg := "同一设备的新连接已通过认证，本连接被平台断开（互踢）"
	go h.notifyAndKick(username, pk.Connect.ClientIdentifier, "new_login", msg)
	return true
}

// notifyAndKick 向设备发布断开原因并踢线（宽限期设计见 kickNoticeGrace）。
// 通知经 broker 进程内 Publish 直接投递（平台账号语义，绕过 ACL）。
func (h *WendaoHook) notifyAndKick(deviceID, excludeClientID, reason, msg string) {
	notice, _ := marshalKickedNotice(reason, msg)
	// 先发布（QoS1 入队）再等宽限，保证旧会话尽量先收到通知再断开
	if err := h.broker.srv.Publish(protocol.TopicKicked(deviceID), notice, false, 1); err != nil {
		log.Printf("broker kick: publish kicked notice to %s failed: %v", deviceID, err)
	}
	if kickNoticeGrace > 0 {
		time.Sleep(kickNoticeGrace)
	}
	kicked, err := h.broker.KickByUsername(deviceID, excludeClientID)
	if err != nil {
		log.Printf("broker kick %s failed: %v", deviceID, err)
	} else if len(kicked) > 0 {
		log.Printf("broker kicked %d session(s) of %s: %v", len(kicked), deviceID, kicked)
	}
}

// authBootstrap 一型一密引导连接认证（逻辑平移自 handler.authBootstrap，逐项一致）。
// 在 CONNECT 阶段完成全部强校验，缩小动态注册接口的探测面。
func (h *WendaoHook) authBootstrap(sn, productKey, password string) bool {
	product, err := h.broker.store.GetProductByKey(productKey)
	if err != nil {
		log.Printf("broker auth(bootstrap): product not found key=%s sn=%s", productKey, sn)
		return false
	}
	if !product.DynRegEnabled {
		log.Printf("broker auth(bootstrap): dynreg disabled key=%s sn=%s", productKey, sn)
		return false
	}
	if !cryptopkg.CheckPassword(product.ProductSecret, password) {
		log.Printf("broker auth(bootstrap): bad product secret key=%s sn=%s", productKey, sn)
		return false
	}
	dev, err := h.broker.store.GetDevice(sn)
	if err != nil {
		log.Printf("broker auth(bootstrap): sn not preregistered key=%s sn=%s", productKey, sn)
		return false
	}
	if dev.ProductID != product.ID {
		log.Printf("broker auth(bootstrap): product mismatch sn=%s got_product=%d", sn, dev.ProductID)
		return false
	}
	if !dev.Enabled {
		return false
	}
	if dev.DeviceSecret != "" {
		return false
	}
	log.Printf("broker auth(bootstrap): allow sn=%s product=%s", sn, productKey)
	return true
}

// OnACLCheck 授权检查（publish/subscribe 均回调，返回 bool）。
// 语义平移自原 handler.EMQXACL：
//   - 平台自身账号：全放行（需跨设备发布 kicked、订阅 +/data 等，超管语义）；
//   - 引导连接：仅允许自身 SN 的注册请求(pub)/应答(sub)两个字面主题；
//   - 普通设备：publish 白名单（data/control/ack/ping/ack/ota/ack/ota/progress/status/peer），
//     subscribe 本人前缀全放行（含通配）。
func (h *WendaoHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	username := hookClientUsername(cl)
	if username == "" {
		return false
	}

	// 平台自身账号全放行
	if username == h.broker.cfg.ServerUsername {
		return true
	}

	// 一型一密引导连接：仅注册双主题
	if sn, _, isBootstrap := protocol.ParseBootstrapUsername(username); isBootstrap {
		if write {
			return topic == protocol.TopicRegisterReq(sn)
		}
		return topic == protocol.TopicRegisterResp(sn)
	}

	parts := strings.Split(topic, "/")
	// 期望形如 wendao/{deviceID}/...；第二段必须等于本人 username（防越权/占位符）
	if len(parts) < 3 || parts[0] != "wendao" || parts[1] != username {
		return false
	}
	suffix := strings.Join(parts[2:], "/")

	if write {
		allowedPub := map[string]bool{
			"data":         true,
			"control/ack":  true,
			"ping/ack":     true,
			"ota/ack":      true,
			"ota/progress": true,
			"status":       true,
			"peer":         true,
		}
		return allowedPub[suffix]
	}
	// 订阅：本人前缀下全放行（第二段已校验等于本人，订阅展开只会命中自己名下主题，
	// 无越权面；放行以免 MQTTX 等恢复历史通配订阅时被拒）。发布仍走严格白名单。
	return true
}

// OnSessionEstablished 会话建立（认证通过且订阅处理完成）：
// 普通设备置在线并广播；同时登记 sessionIndex 供互踢。
func (h *WendaoHook) OnSessionEstablished(cl *mqtt.Client, pk packets.Packet) {
	username := hookClientUsername(cl)
	clientID := cl.ID
	if username == "" {
		return
	}
	h.broker.sessions.Add(username, clientID, cl)

	if username == h.broker.cfg.ServerUsername || isBootstrapUsername(username) {
		return // 平台账号与引导连接不参与设备在线状态
	}
	h.setDeviceOnline(username, clientID, true)
}

// OnDisconnect 连接断开（主动断开/互踢/keepalive 超时等一切原因）：
// 普通设备置离线并广播；从 sessionIndex 注销。
func (h *WendaoHook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	username := hookClientUsername(cl)
	clientID := cl.ID
	h.broker.sessions.Remove(username, clientID)

	if username == "" || username == h.broker.cfg.ServerUsername || isBootstrapUsername(username) {
		return
	}
	h.setDeviceOnline(username, clientID, false)
	if err != nil && expire {
		log.Printf("broker disconnect: %s (client %s) session expired", username, clientID)
	}
}

// setDeviceOnline 更新设备在线状态并广播前端（逻辑平移自原 handleClientEvent）。
// 仅处理已注册设备（认证失败的连接不会产生会话事件，双保险）。
func (h *WendaoHook) setDeviceOnline(deviceID, clientID string, online bool) {
	if _, err := h.broker.store.GetDevice(deviceID); err != nil {
		return
	}
	if err := h.broker.store.SetDeviceRuntimeStatus(deviceID, online); err != nil {
		log.Printf("broker status: set %s online=%v error: %v", deviceID, online, err)
		return
	}
	log.Printf("broker status: %s %s (client %s)", deviceID,
		map[bool]string{true: "online", false: "offline"}[online], clientID)

	if h.broker.bus != nil {
		if dev, err := h.broker.store.GetDevice(deviceID); err == nil && dev.TenantID != 0 {
			h.broker.bus.BroadcastToTenant(dev.TenantID, eventsMessage("device_status", map[string]interface{}{
				"device_id": deviceID, "online": online,
			}))
		}
	}
}
