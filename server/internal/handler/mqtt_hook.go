package handler

import (
	"log"
	"net/http"
	"strings"
	"time"

	"wendaoiotpannel/internal/protocol"
	cryptopkg "wendaoiotpannel/pkg/crypto"

	"github.com/gin-gonic/gin"
)

// mqttAuthSecret 由 main 注入（config server.mqtt_auth_secret / WQ_MQTT_AUTH_SECRET）。
var mqttAuthSecret string

func SetMQTTAuthSecret(s string) {
	mqttAuthSecret = s
}

// SessionKicker 互踢能力（由 mqtt.Client 实现；未配置 EMQX API 时为 nil）。
type SessionKicker interface {
	KickDeviceSessions(deviceID string) ([]string, error)
}

// KickedNotifier 被踢通知能力：主动断开前向设备发布原因（由 mqtt.Client 实现）。
type KickedNotifier interface {
	NotifyKicked(deviceID, reason, msg string)
}

// SetSessionKicker 注入互踢实现（main 启动时调用；nil 表示禁用）。
var sessionKicker SessionKicker

func SetSessionKicker(k SessionKicker) {
	sessionKicker = k
}

// SetKickedNotifier 注入被踢通知实现（main 启动时调用；nil 表示禁用）。
var kickedNotifier KickedNotifier

func SetKickedNotifier(n KickedNotifier) {
	kickedNotifier = n
}

// kickNoticeGrace 通知已入队到执行踢线之间预留的投递宽限：
// 让 EMQX 有时间把 QoS1 kicked 消息下发给被踢的在线会话（clean session=true 时
// 消息不持久化，此宽限是其收到原因的唯一窗口；clean session=false 重连后仍可补收）。
var kickNoticeGrace = 600 * time.Millisecond

// notifyAndKick 先向设备发布断开原因（若通知器可用），再执行互踢。
// 通知失败只记日志，不影响踢出流程；kicker 未注入则仅通知不踢。
func notifyAndKick(deviceID, reason, msg string) {
	if kickedNotifier != nil {
		kickedNotifier.NotifyKicked(deviceID, reason, msg)
	}
	if kickNoticeGrace > 0 {
		time.Sleep(kickNoticeGrace)
	}
	if sessionKicker != nil {
		kicked, err := sessionKicker.KickDeviceSessions(deviceID)
		if err != nil {
			log.Printf("mqtt auth: kick old sessions for %s failed: %v", deviceID, err)
		} else if len(kicked) > 0 {
			log.Printf("mqtt auth: kicked %d old session(s) of device %s: %v", len(kicked), deviceID, kicked)
		}
	}
}

// EMQX 认证/ACL 回调请求体（EMQX HTTP Auth/ACL webhook 字段）。
type emqxAuthReq struct {
	ClientID string `json:"clientid"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type emqxACLReq struct {
	ClientID string `json:"clientid"`
	Username string `json:"username"`
	Topic    string `json:"topic"`
	Action   string `json:"action"` // publish / subscribe
}

// checkHookSecret 校验 EMQX 回调解入方持有共享密钥。
func checkHookSecret(c *gin.Context) bool {
	got := c.GetHeader("X-Auth-Key")
	if got == "" {
		got = c.Query("secret")
	}
	if mqttAuthSecret == "" || got != mqttAuthSecret {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"result": "deny", "reason": "bad hook secret"})
		return false
	}
	return true
}

// EMQXAuthenticate 设备接入认证：username=设备ID，password=一机一密明文，bcrypt 校验 + 启用状态。
func (h *Handler) EMQXAuthenticate(c *gin.Context) {
	if !checkHookSecret(c) {
		return
	}
	var req emqxAuthReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "bad request"})
		return
	}

	// 一型一密引导连接：username = "{SN}&{ProductKey}"，password = 产品密钥明文。
	// 仅当：产品存在且开启动态注册、产品密钥 bcrypt 匹配、SN 已预录到该产品、
	// 设备启用且尚未激活（无一机一密）时放行；不触发互踢。
	if sn, productKey, isBootstrap := protocol.ParseBootstrapUsername(req.Username); isBootstrap {
		h.authBootstrap(c, sn, productKey, req.Password)
		return
	}

	dev, err := h.store.GetDeviceAuthRecord(req.Username)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "device not found"})
		return
	}
	if !dev.Enabled {
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "device disabled"})
		return
	}
	if dev.DeviceSecret == "" {
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "device secret not provisioned"})
		return
	}
	if !cryptopkg.CheckPassword(dev.DeviceSecret, req.Password) {
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "bad credentials"})
		return
	}

	// 同设备重复连接互踢：认证已通过，此时新连接尚未注册完成，
	// 先向旧连接发布被踢原因（wendao/{id}/kicked，QoS1 持久化、重连可收），
	// 再全部踢掉（"新踢旧"）。踢失败仅记日志不拒绝登录。
	notifyAndKick(req.Username, "new_login",
		"同一设备的新连接已通过认证，本连接被平台断开（互踢）")

	c.JSON(http.StatusOK, gin.H{"result": "allow", "is_superuser": false})
}

// EMQXACL 授权：普通设备只能 pub/sub 自己 wendao/{deviceID}/... 的允许主题。
// 引导连接（username 含 '&'）仅允许其自身 SN 的注册双主题，其余一律 deny（EMQX 配合踢线）。
func (h *Handler) EMQXACL(c *gin.Context) {
	if !checkHookSecret(c) {
		return
	}
	var req emqxACLReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"result": "deny"})
		return
	}

	// 一型一密引导连接 ACL 特判
	if sn, _, isBootstrap := protocol.ParseBootstrapUsername(req.Username); isBootstrap {
		c.JSON(http.StatusOK, gin.H{"result": h.bootstrapACLResult(req.Action, sn, req.Topic)})
		return
	}

	topic := req.Topic
	parts := strings.Split(topic, "/")
	// 期望形如 wendao/{deviceID}/...
	if len(parts) < 3 || parts[0] != "wendao" || parts[1] != req.Username {
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "cross-device topic"})
		return
	}
	suffix := strings.Join(parts[2:], "/")

	// publish 允许后缀：data、control/ack、ping/ack(在线探活应答)、ota/ack、ota/progress、status、peer（设备间消息）
	allowedPub := map[string]bool{
		"data":         true,
		"control/ack":  true,
		"ping/ack":     true,
		"ota/ack":      true,
		"ota/progress": true,
		"status":       true,
		"peer":         true,
	}
	var allow bool
	if req.Action == "publish" {
		allow = allowedPub[suffix]
	} else {
		// 订阅：本人前缀下全放行（data/ack、control、ota、peer/ack、inbox 等字面主题 +
		// 任意通配符过滤器均无需白名单）。第二段已校验等于本人设备ID，订阅展开只会命中
		// 自己名下的主题，无越权面；放行以免 MQTTX 等客户端恢复历史订阅时被
		// no_match=deny + deny_action=disconnect 踢线（Not authorized 135）。
		// 发布（publish）不受此影响，仍保持严格白名单。
		allow = true
	}
	if allow {
		c.JSON(http.StatusOK, gin.H{"result": "allow"})
	} else {
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "topic not permitted"})
	}
}

// authBootstrap 一型一密引导连接认证（详见 EMQXAuthenticate 内分支注释）。
// 在 CONNECT 阶段完成全部强校验，未预录/错产品密钥/已激活等一律拒绝，缩小探测面。
func (h *Handler) authBootstrap(c *gin.Context, sn, productKey, password string) {
	product, err := h.store.GetProductByKey(productKey)
	if err != nil {
		log.Printf("mqtt auth(bootstrap): product not found key=%s sn=%s", productKey, sn)
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "product not found"})
		return
	}
	if !product.DynRegEnabled {
		log.Printf("mqtt auth(bootstrap): dynreg disabled key=%s sn=%s", productKey, sn)
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "dynamic registration disabled"})
		return
	}
	if !cryptopkg.CheckPassword(product.ProductSecret, password) {
		log.Printf("mqtt auth(bootstrap): bad product secret key=%s sn=%s", productKey, sn)
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "bad product secret"})
		return
	}
	dev, err := h.store.GetDevice(sn)
	if err != nil {
		log.Printf("mqtt auth(bootstrap): sn not preregistered key=%s sn=%s", productKey, sn)
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "sn not preregistered"})
		return
	}
	if dev.ProductID != product.ID {
		log.Printf("mqtt auth(bootstrap): product mismatch sn=%s got_product=%d", sn, dev.ProductID)
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "product mismatch"})
		return
	}
	if !dev.Enabled {
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "device disabled"})
		return
	}
	if dev.DeviceSecret != "" {
		c.JSON(http.StatusOK, gin.H{"result": "deny", "reason": "already activated"})
		return
	}
	log.Printf("mqtt auth(bootstrap): allow sn=%s product=%s", sn, productKey)
	c.JSON(http.StatusOK, gin.H{"result": "allow", "is_superuser": false})
}

// bootstrapACLResult 引导连接仅允许自身 SN 的注册请求(pub)/应答(sub)两个字面主题；
// 通配符或他人 SN 因字面不等直接 deny（EMQX deny_action=disconnect 会踢线）。
func (h *Handler) bootstrapACLResult(action, sn, topic string) string {
	switch action {
	case "publish":
		if topic == protocol.TopicRegisterReq(sn) {
			return "allow"
		}
	case "subscribe":
		if topic == protocol.TopicRegisterResp(sn) {
			return "allow"
		}
	}
	log.Printf("mqtt acl(bootstrap): deny action=%s sn=%s topic=%s", action, sn, topic)
	return "deny"
}
