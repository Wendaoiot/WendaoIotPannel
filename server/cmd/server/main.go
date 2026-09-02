package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"wendaoiotpannel/internal/config"
	"wendaoiotpannel/internal/handler"
	mqttclient "wendaoiotpannel/internal/mqtt"
	"wendaoiotpannel/internal/store"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSHub struct {
	clients map[*websocket.Conn]bool
	mu      sync.RWMutex
}

var wsHub = &WSHub{clients: make(map[*websocket.Conn]bool)}

func (h *WSHub) Add(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()
}

func (h *WSHub) Remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

func (h *WSHub) Broadcast(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			go func(c *websocket.Conn) {
				h.mu.Lock()
				delete(h.clients, c)
				h.mu.Unlock()
				c.Close()
			}(conn)
		}
	}
}

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	handler.InitJWTSecret(cfg.Server.JWT)

	s, err := store.New(cfg.MySQLDSN())
	if err != nil {
		log.Fatalf("connect mysql: %v", err)
	}
	if err := s.AutoMigrate(); err != nil {
		log.Fatalf("auto migrate: %v", err)
	}

	mc, err := mqttclient.New(mqttclient.Config{
		Broker:   cfg.MQTT.Broker,
		ClientID: cfg.MQTT.ClientID,
		Username: cfg.MQTT.Username,
		Password: cfg.MQTT.Password,
	}, s)
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

	// 启动离线检测定时任务（每1分钟检查一次，超过1分钟未活跃则标记为离线）
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		for range ticker.C {
			if err := s.MarkOfflineDevices(1 * time.Minute); err != nil {
				log.Printf("mark offline devices error: %v", err)
			}
		}
	}()

	h := handler.New(s, mc)

	handler.SeedAdminUsers(s)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	api := r.Group("/api/v1")
	api.GET("/health", h.HealthCheck)
	api.POST("/login", h.Login)

	auth := api.Group("", handler.AuthMiddleware(s))
	{
		auth.GET("/dashboard/stats", h.GetDashboardStats)
		auth.GET("/projects/:id/data", h.GetProjectData)

		auth.POST("/tenants", h.CreateTenant)
		auth.GET("/tenants", h.ListTenants)
		auth.PUT("/tenants/:id", h.UpdateTenant)
		auth.DELETE("/tenants/:id", h.DeleteTenant)

		auth.POST("/projects", h.CreateProject)
		auth.GET("/projects", h.ListProjects)
		auth.PUT("/projects/:id", h.UpdateProject)
		auth.DELETE("/projects/:id", h.DeleteProject)

		auth.POST("/devices", h.CreateDevice)
		auth.GET("/devices", h.ListDevices)
		auth.GET("/devices/:deviceId", h.GetDevice)
		auth.PUT("/devices/:deviceId", h.UpdateDevice)
		auth.DELETE("/devices/:deviceId", h.DeleteDevice)

		auth.POST("/devices/:deviceId/tags", h.CreateDeviceTag)
		auth.GET("/devices/:deviceId/tags", h.ListDeviceTags)
		auth.DELETE("/devices/:deviceId/tags", h.DeleteDeviceTag)

		auth.POST("/projects/:id/tags", h.CreateProjectTag)
		auth.GET("/projects/:id/tags", h.ListProjectTags)
		auth.DELETE("/projects/:id/tags", h.DeleteProjectTag)

		auth.GET("/devices/:deviceId/data", h.GetDeviceData)
		auth.POST("/devices/:deviceId/control", h.SendControl)

		auth.GET("/control-logs", h.ListControlLogs)

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

	auth.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		wsHub.Add(conn)
		defer func() {
			wsHub.Remove(conn)
			conn.Close()
		}()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	})

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("server start on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server run: %v", err)
	}
}
