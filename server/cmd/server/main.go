package main

import (
	"fmt"
	"log"
	"time"

	"wendaoiotpannel/internal/config"
	"wendaoiotpannel/internal/events"
	"wendaoiotpannel/internal/handler"
	"wendaoiotpannel/internal/model"
	mqttclient "wendaoiotpannel/internal/mqtt"
	"wendaoiotpannel/internal/store"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config invalid: %v", err)
	}
	log.Printf("config loaded: server.port=%d mysql=%s:%d/%s mqtt=%s",
		cfg.Server.Port, cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database, cfg.MQTT.Broker)

	handler.InitJWTSecret(cfg.Server.JWT)
	handler.SetMQTTAuthSecret(cfg.Server.MQTTAuthSecret)

	s, err := store.New(cfg.MySQLDSN())
	if err != nil {
		log.Fatalf("connect mysql: %v", err)
	}
	if err := s.AutoMigrate(); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	bus := events.NewBus()

	mc, err := mqttclient.New(mqttclient.Config{
		Broker:   cfg.MQTT.Broker,
		ClientID: cfg.MQTT.ClientID,
		Username: cfg.MQTT.Username,
		Password: cfg.MQTT.Password,
	}, s, bus)
	if err != nil {
		log.Fatalf("create mqtt client: %v", err)
	}
	if err := mc.Connect(); err != nil {
		log.Fatalf("connect mqtt broker: %v", err)
	}
	if err := mc.Subscribe(); err != nil {
		log.Fatalf("subscribe mqtt: %v", err)
	}
	log.Println("MQTT connected and subscribed")

	// EMQX REST 客户端（同设备重复连接互踢）。API Key 未配置时功能静默禁用。
	if cfg.EMQX.APIBase != "" && cfg.EMQX.APIKey != "" && cfg.EMQX.APISecret != "" {
		admin := mqttclient.NewEMQXAdmin(cfg.EMQX.APIBase, cfg.EMQX.APIKey, cfg.EMQX.APISecret)
		mc.SetEMQXAdmin(admin)
		handler.SetSessionKicker(mc)
		handler.SetKickedNotifier(mc) // 被踢通知：断开前向 wendao/{id}/kicked 发布原因
		log.Printf("EMQX admin api enabled: %s (device session kick active)", cfg.EMQX.APIBase)
	} else {
		log.Println("EMQX admin api not configured: duplicate device sessions will NOT be kicked")
	}

	// 在线状态维护（设备级模式，系统默认 connection）：
	//   connection — 上下线由 EMQX $SYS connected/disconnected 事件实时驱动，扫描不回收；
	//   report     — 周期扫描按设备 offline_timeout_sec(>0) 或全局 offline_timeout_sec
	//                判"超时未上报离线"；全局与设备超时都<=0 时该设备不做周期判定；
	//   ping       — 周期探活，超时不应答由扫描回收。
	// 兜底说明：connection 设备依赖断开事件，若 broker 宕机期间事件丢失会残留在线直到事件恢复。
	offlineTimeout := time.Duration(cfg.Device.OfflineTimeoutSec) * time.Second
	scanInterval := time.Duration(cfg.Device.ScanIntervalSec) * time.Second
	log.Printf("device online mode: %s (offline_timeout=%s scan=%s; per-device override supported)",
		cfg.Device.OnlineMode, offlineTimeout, scanInterval)
	go func() {
		ticker := time.NewTicker(scanInterval)
		defer ticker.Stop()
		for range ticker.C {
			if err := s.MarkOfflineDevices(offlineTimeout); err != nil {
				log.Printf("mark offline devices error: %v", err)
			}
		}
	}()

	// ping 在线判定模式：周期性向有效模式=ping 的启用设备发布探活 wendao/{id}/ping。
	// 设备应答 wendao/{id}/ping/ack 会刷新 last_active（mqtt.handlePingAck）；
	// 超时未应答由上方离线扫描回收；MQTT 断开事件始终立即判离线（三模式通用前提）。
	pingInterval := time.Duration(cfg.Device.PingIntervalSec) * time.Second
	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for range ticker.C {
			ids, err := s.ListPingModeDevices()
			if err != nil {
				log.Printf("list ping-mode devices error: %v", err)
				continue
			}
			for _, id := range ids {
				pingID := fmt.Sprintf("ping-%d", time.Now().UnixNano())
				if err := mc.PublishPing(id, pingID); err != nil {
					log.Printf("publish ping to %s error: %v", id, err)
				}
			}
		}
	}()

	// 控制指令超时回收：每 15s 将超过 30s 未 ack 的下发标记为超时
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if n, err := s.MarkControlTimeout(30 * time.Second); err != nil {
				log.Printf("mark control timeout error: %v", err)
			} else if n > 0 {
				log.Printf("marked %d control commands timeout", n)
			}
		}
	}()

	h := handler.New(s, mc, bus)
	handler.SeedAdminUsers(s)

	r := gin.Default()
	r.Use(cors.New(buildCORS(cfg)))

	api := r.Group("/api/v1")
	api.GET("/health", h.HealthCheck)
	api.POST("/login", h.Login)

	// EMQX 认证/ACL 回调（非用户 JWT，用共享密钥保护）
	api.POST("/mqtt/auth", h.EMQXAuthenticate)
	api.POST("/mqtt/acl", h.EMQXACL)

	auth := api.Group("", handler.AuthMiddleware(s))
	{
		auth.GET("/dashboard/stats", h.GetDashboardStats)
		auth.GET("/projects/:id/data", h.GetProjectData)

		// 租户管理：仅超管（路由级强制）
		auth.POST("/tenants", handler.RequireRole(model.RoleSuperAdmin), h.CreateTenant)
		auth.GET("/tenants", handler.RequireRole(model.RoleSuperAdmin), h.ListTenants)
		auth.PUT("/tenants/:id", handler.RequireRole(model.RoleSuperAdmin), h.UpdateTenant)
		auth.DELETE("/tenants/:id", handler.RequireRole(model.RoleSuperAdmin), h.DeleteTenant)

		auth.POST("/projects", h.CreateProject)
		auth.GET("/projects", h.ListProjects)
		auth.PUT("/projects/:id", h.UpdateProject)
		auth.DELETE("/projects/:id", h.DeleteProject)

		auth.POST("/devices", h.CreateDevice)
		auth.GET("/devices", h.ListDevices)
		auth.GET("/devices/:deviceId", h.GetDevice)
		auth.PUT("/devices/:deviceId", h.UpdateDevice)
		auth.DELETE("/devices/:deviceId", h.DeleteDevice)
		auth.PUT("/devices/:deviceId/enabled", h.SetDeviceEnabled)
		auth.POST("/devices/:deviceId/secret/reset", h.ResetDeviceSecret)

		auth.POST("/devices/:deviceId/tags", h.CreateDeviceTag)
		auth.GET("/devices/:deviceId/tags", h.ListDeviceTags)
		auth.DELETE("/devices/:deviceId/tags", h.DeleteDeviceTag)

		auth.POST("/projects/:id/tags", h.CreateProjectTag)
		auth.GET("/projects/:id/tags", h.ListProjectTags)
		auth.DELETE("/projects/:id/tags", h.DeleteProjectTag)

		auth.GET("/devices/:deviceId/data", h.GetDeviceData)
		// 数据删除：仅超管（图表页/数据页共用）
		auth.DELETE("/devices/:deviceId/data", handler.RequireRole(model.RoleSuperAdmin), h.DeleteDeviceData)
		auth.POST("/devices/:deviceId/control", h.SendControl)
		auth.GET("/devices/:deviceId/control/:msgId", h.GetControlStatus)

		// 自定义控制命令（项目维度）
		auth.GET("/projects/:id/commands", h.ListControlCommands)
		auth.POST("/projects/:id/commands", h.CreateControlCommand)
		auth.PUT("/commands/:id", h.UpdateControlCommand)
		auth.DELETE("/commands/:id", h.DeleteControlCommand)

		auth.GET("/control-logs", h.ListControlLogs)

		// 设备间通信（D2D）
		auth.GET("/devices/:deviceId/peer/messages", h.ListPeerMessages)
		auth.GET("/peer/allows", h.ListPeerAllows)
		auth.POST("/peer/allows", h.CreatePeerAllow)
		auth.DELETE("/peer/allows/:id", h.DeletePeerAllow)

		auth.POST("/firmwares", h.CreateFirmware)
		auth.GET("/firmwares", h.ListFirmwares)
		auth.DELETE("/firmwares", h.DeleteFirmware)
		auth.GET("/firmwares/latest", h.GetDeviceFirmware)

		auth.POST("/ota/tasks", h.CreateOTATask)
		auth.GET("/ota/tasks", h.ListOTATasks)
		auth.GET("/ota/logs", h.GetOTALogs)

		auth.GET("/users", h.ListUsers)
		auth.PUT("/users/password", h.ChangePassword)
		auth.PUT("/users/:id/password", h.AdminResetUserPassword)
		auth.DELETE("/users/:id", h.DeleteUser)
	}

	// WebSocket 实时推送：必须在 auth 组之外。
	// 浏览器 WS 握手无法自定义 Authorization 头，AuthMiddleware 会直接 401；
	// ServeWS 自带 query/Sec-WebSocket-Protocol token 鉴权（校验 token_version）。
	api.GET("/ws", h.ServeWS)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("server start on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server run: %v", err)
	}
}

func buildCORS(cfg *config.Config) cors.Config {
	origins := cfg.Server.CorsOrigins
	if len(origins) == 0 {
		// 未配置则放行本机开发来源；生产应通过 config/env 明确白名单。
		origins = []string{
			"http://localhost:3000", "http://127.0.0.1:3000",
			"http://localhost:3001", "http://127.0.0.1:3001",
		}
	}
	c := cors.DefaultConfig()
	c.AllowOrigins = origins
	c.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	c.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	c.ExposeHeaders = []string{"Content-Length", "Content-Disposition"}
	c.AllowCredentials = true
	c.MaxAge = 12 * time.Hour
	return c
}
