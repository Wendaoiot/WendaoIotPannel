package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"wendaoiotpannel/internal/model"
	"wendaoiotpannel/internal/protocol"
	"wendaoiotpannel/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	store *store.Store
	mqtt  MQTTPublisher
}

type MQTTPublisher interface {
	PublishControl(deviceID string, cmd *protocol.DownlinkRequest) error
	PublishRaw(topic string, payload []byte) error
}

func New(s *store.Store, mqtt MQTTPublisher) *Handler {
	return &Handler{store: s, mqtt: mqtt}
}

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Msg: "ok", Data: data})
}

func fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Response{Code: code, Msg: msg})
}

func (h *Handler) HealthCheck(c *gin.Context) {
	success(c, gin.H{"status": "ok", "time": time.Now().Unix()})
}

func getAuthInfo(c *gin.Context) (role string, tenantID *uint) {
	role = c.GetString("role")
	val, exists := c.Get("tenant_id")
	if exists && val != nil {
		if tid, ok := val.(*uint); ok {
			tenantID = tid
		}
	}
	return
}

func (h *Handler) assertProjectBelongsToTenant(projectID uint, tenantID *uint) bool {
	if tenantID == nil {
		return true
	}
	p, err := h.store.GetProjectByID(projectID)
	if err != nil {
		return false
	}
	return p.TenantID == *tenantID
}

// Tenant

func (h *Handler) CreateTenant(c *gin.Context) {
	var req struct {
		Name     string `json:"name" binding:"required"`
		AdminPwd string `json:"admin_pwd"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}
	t := &model.Tenant{Name: req.Name}
	if err := h.store.CreateTenant(t); err != nil {
		fail(c, -1, err.Error())
		return
	}

	pwd := req.AdminPwd
	if pwd == "" {
		pwd = "123456"
	}
	adminUser := &model.AdminUser{
		Username: t.Name + "_admin",
		Password: hashPassword(pwd),
		Role:     model.RoleTenantAdmin,
		TenantID: &t.ID,
	}
	if err := h.store.CreateAdminUser(adminUser); err != nil {
		adminUser.Username = fmt.Sprintf("tenant_%d", t.ID)
		if err2 := h.store.CreateAdminUser(adminUser); err2 != nil {
		}
	}

	success(c, gin.H{
		"tenant":     t,
		"admin_user": adminUser.Username,
		"admin_pwd":  pwd,
	})
}

func (h *Handler) ListTenants(c *gin.Context) {
	tenants, err := h.store.ListTenants()
	if err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, tenants)
}

func (h *Handler) UpdateTenant(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, -1, "invalid id")
		return
	}
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}
	if err := h.store.UpdateTenant(uint(id), req.Name); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, nil)
}

func (h *Handler) DeleteTenant(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, -1, "invalid id")
		return
	}
	if err := h.store.DeleteTenant(uint(id)); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, nil)
}

// Project

func (h *Handler) CreateProject(c *gin.Context) {
	role, tenantID := getAuthInfo(c)
	var req struct {
		TenantID uint   `json:"tenant_id"`
		Name     string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}
	if role == model.RoleTenantAdmin {
		if tenantID == nil {
			fail(c, -1, "租户信息异常")
			return
		}
		req.TenantID = *tenantID
	}
	if req.TenantID == 0 {
		fail(c, -1, "请指定所属租户")
		return
	}
	p := &model.Project{TenantID: req.TenantID, Name: req.Name}
	if err := h.store.CreateProject(p); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, p)
}

func (h *Handler) ListProjects(c *gin.Context) {
	role, tenantID := getAuthInfo(c)
	if role == model.RoleTenantAdmin && tenantID == nil {
		fail(c, -1, "租户信息异常")
		return
	}

	if tenantID != nil {
		projects, err := h.store.ListProjectsByTenant(*tenantID)
		if err != nil {
			fail(c, -1, err.Error())
			return
		}
		success(c, projects)
		return
	}

	var req struct {
		TenantID uint `form:"tenant_id"`
	}
	c.ShouldBindQuery(&req)

	var projects []model.Project
	var err error
	if req.TenantID > 0 {
		projects, err = h.store.ListProjectsByTenant(req.TenantID)
	} else {
		projects, err = h.store.ListAllProjects()
	}
	if err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, projects)
}

func (h *Handler) UpdateProject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, -1, "invalid id")
		return
	}
	_, tenantID := getAuthInfo(c)
	if !h.assertProjectBelongsToTenant(uint(id), tenantID) {
		fail(c, 403, "无权操作此项目")
		return
	}
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}
	if err := h.store.UpdateProject(uint(id), req.Name); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, nil)
}

func (h *Handler) DeleteProject(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, -1, "invalid id")
		return
	}
	_, tenantID := getAuthInfo(c)
	if !h.assertProjectBelongsToTenant(uint(id), tenantID) {
		fail(c, 403, "无权操作此项目")
		return
	}
	if err := h.store.DeleteProject(uint(id)); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, nil)
}

// Device

func (h *Handler) CreateDevice(c *gin.Context) {
	_, tenantID := getAuthInfo(c)
	var req struct {
		ID        string `json:"id" binding:"required"`
		ProjectID uint   `json:"project_id" binding:"required"`
		Name      string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}
	if !h.assertProjectBelongsToTenant(req.ProjectID, tenantID) {
		fail(c, 403, "无权在此项目中创建设备")
		return
	}
	d := &model.Device{
		ID:        req.ID,
		ProjectID: req.ProjectID,
		Name:      req.Name,
		Status:    model.DeviceStatusOffline,
	}
	if err := h.store.CreateDevice(d); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, d)
}

func (h *Handler) ListDevices(c *gin.Context) {
	_, tenantID := getAuthInfo(c)
	var req struct {
		ProjectID *uint `form:"project_id"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}

	var devices []model.Device
	if req.ProjectID != nil {
		if !h.assertProjectBelongsToTenant(*req.ProjectID, tenantID) {
			fail(c, 403, "无权查看此项目的设备")
			return
		}
		devices, _ = h.store.ListDevicesByProject(*req.ProjectID)
	} else if tenantID != nil {
		// 未指定项目时，列出当前租户所有项目的设备
		projects, err := h.store.ListProjectsByTenant(*tenantID)
		if err != nil {
			fail(c, -1, err.Error())
			return
		}
		var projectIDs []uint
		for _, p := range projects {
			projectIDs = append(projectIDs, p.ID)
		}
		if len(projectIDs) == 0 {
			success(c, []model.Device{})
			return
		}
		devices, err = h.store.ListDevicesByProjects(projectIDs)
		if err != nil {
			fail(c, -1, err.Error())
			return
		}
	} else {
		// 超级管理员未指定项目，列出所有设备
		devices, _ = h.store.ListDevicesByProjects(nil)
	}

	success(c, devices)
}

func (h *Handler) GetDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")
	_, tenantID := getAuthInfo(c)
	if !h.assertDeviceBelongsToTenant(deviceID, tenantID) {
		fail(c, 403, "无权查看此设备")
		return
	}
	device, err := h.store.GetDevice(deviceID)
	if err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, device)
}

func (h *Handler) UpdateDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")
	_, tenantID := getAuthInfo(c)
	if !h.assertDeviceBelongsToTenant(deviceID, tenantID) {
		fail(c, 403, "无权操作此设备")
		return
	}
	var req struct {
		Name      string `json:"name" binding:"required"`
		ProjectID uint   `json:"project_id" binding:"required"`
		Status    *int   `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}
	if !h.assertProjectBelongsToTenant(req.ProjectID, tenantID) {
		fail(c, 403, "无权将设备迁移到此项目")
		return
	}
	status := model.DeviceStatusOffline
	if req.Status != nil {
		status = *req.Status
	}
	if err := h.store.UpdateDevice(deviceID, req.Name, req.ProjectID, status); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, nil)
}

func (h *Handler) DeleteDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")
	_, tenantID := getAuthInfo(c)
	if !h.assertDeviceBelongsToTenant(deviceID, tenantID) {
		fail(c, 403, "无权操作此设备")
		return
	}
	if err := h.store.DeleteDevice(deviceID); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, nil)
}

// DeviceTag

func (h *Handler) CreateDeviceTag(c *gin.Context) {
	deviceID := c.Param("deviceId")
	var req struct {
		TagKey    string `json:"tag_key" binding:"required"`
		Interface string `json:"interface" binding:"required"`
		Formula   string `json:"formula"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}
	t := &model.DeviceTag{
		DeviceID:  deviceID,
		TagKey:    req.TagKey,
		Interface: req.Interface,
		Formula:   req.Formula,
	}
	if err := h.store.CreateDeviceTag(t); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, t)
}

func (h *Handler) ListDeviceTags(c *gin.Context) {
	deviceID := c.Param("deviceId")
	tags, err := h.store.ListDeviceTags(deviceID)
	if err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, tags)
}

func (h *Handler) DeleteDeviceTag(c *gin.Context) {
	var req struct {
		ID uint `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}
	if err := h.store.DeleteDeviceTag(req.ID); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, nil)
}

// ProjectTag

func (h *Handler) CreateProjectTag(c *gin.Context) {
	projectID := c.Param("id")
	pid, err := strconv.ParseUint(projectID, 10, 64)
	if err != nil {
		fail(c, -1, "invalid project id")
		return
	}
	var req struct {
		TagKey   string `json:"tag_key" binding:"required"`
		TagName  string `json:"tag_name"`
		Unit     string `json:"unit"`
		DataType string `json:"data_type"`
		Writable bool   `json:"writable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}
	if req.TagName == "" {
		req.TagName = req.TagKey
	}
	if req.DataType == "" {
		req.DataType = "number"
	}
	t := &model.ProjectTag{
		ProjectID: uint(pid),
		TagKey:    req.TagKey,
		TagName:   req.TagName,
		Unit:      req.Unit,
		DataType:  req.DataType,
		Writable:  req.Writable,
	}
	if err := h.store.CreateProjectTag(t); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, t)
}

func (h *Handler) ListProjectTags(c *gin.Context) {
	projectID := c.Param("id")
	pid, err := strconv.ParseUint(projectID, 10, 64)
	if err != nil {
		fail(c, -1, "invalid project id")
		return
	}
	tags, err := h.store.ListProjectTags(uint(pid))
	if err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, tags)
}

func (h *Handler) DeleteProjectTag(c *gin.Context) {
	var req struct {
		ID uint `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}
	if err := h.store.DeleteProjectTag(req.ID); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, nil)
}

// DeviceData

func (h *Handler) GetDeviceData(c *gin.Context) {
	deviceID := c.Param("deviceId")
	limit := 100
	offset := 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}
	if limit > 500 {
		limit = 500
	}
	data, total, err := h.store.ListDeviceData(deviceID, limit, offset)
	if err != nil {
		fail(c, -1, err.Error())
		return
	}

	type dataItem struct {
		ID        uint                   `json:"id"`
		DeviceID  string                 `json:"device_id"`
		MsgID     string                 `json:"msg_id"`
		Ts        int64                  `json:"ts"`
		Version   string                 `json:"version"`
		Data      map[string]interface{} `json:"data"`
		CreatedAt interface{}            `json:"created_at"`
	}
	result := make([]dataItem, 0, len(data))
	for _, d := range data {
		var parsed map[string]interface{}
		json.Unmarshal([]byte(d.Data), &parsed)
		result = append(result, dataItem{
			ID:        d.ID,
			DeviceID:  d.DeviceID,
			MsgID:     d.MsgID,
			Ts:        d.Ts,
			Version:   d.Version,
			Data:      parsed,
			CreatedAt: d.CreatedAt,
		})
	}
	success(c, gin.H{
		"list":   result,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// Control

func (h *Handler) SendControl(c *gin.Context) {
	deviceID := c.Param("deviceId")
	_, tenantID := getAuthInfo(c)
	if !h.assertDeviceBelongsToTenant(deviceID, tenantID) {
		fail(c, 403, "无权操作此设备")
		return
	}
	var req struct {
		Tags map[string]float64 `json:"tags" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}

	dev, err := h.store.GetDevice(deviceID)
	if err != nil {
		fail(c, -1, "device not found")
		return
	}
	if dev.Status == model.DeviceStatusInactive {
		fail(c, -1, "设备未激活，无法下发指令")
		return
	}

	msgID := uuid.New().String()
	cmd := &protocol.DownlinkRequest{
		ID:   msgID,
		Ts:   time.Now().UnixMilli(),
		Tags: req.Tags,
	}

	tagsJSON, _ := json.Marshal(req.Tags)
	log := &model.ControlLog{
		DeviceID: deviceID,
		MsgID:    msgID,
		Tags:     string(tagsJSON),
	}
	if err := h.store.CreateControlLog(log); err != nil {
	}

	if err := h.mqtt.PublishControl(deviceID, cmd); err != nil {
		fail(c, -1, err.Error())
		return
	}

	success(c, gin.H{"msg_id": msgID})
}

func (h *Handler) ListControlLogs(c *gin.Context) {
	deviceID := c.Query("device_id")
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}
	logs, total, err := h.store.ListControlLogs(deviceID, limit, offset)
	if err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, gin.H{"list": logs, "total": total})
}

// Dashboard

func (h *Handler) GetDashboardStats(c *gin.Context) {
	_, tenantID := getAuthInfo(c)
	stats, err := h.store.GetDashboardStats(tenantID)
	if err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, stats)
}

// ProjectData 项目数据聚合
func (h *Handler) GetProjectData(c *gin.Context) {
	projectID := c.Param("id")
	pid, err := strconv.ParseUint(projectID, 10, 64)
	if err != nil {
		fail(c, -1, "invalid project id")
		return
	}

	projTags, err := h.store.ListProjectTags(uint(pid))
	if err != nil {
		fail(c, -1, err.Error())
		return
	}

	devices, err := h.store.ListDevicesByProject(uint(pid))
	if err != nil {
		fail(c, -1, err.Error())
		return
	}

	deviceTagMap := make(map[string]map[string]model.DeviceTag)
	for _, d := range devices {
		tags, _ := h.store.ListDeviceTags(d.ID)
		tm := make(map[string]model.DeviceTag)
		for _, t := range tags {
			tm[t.TagKey] = t
		}
		deviceTagMap[d.ID] = tm
	}

	type DeviceValue struct {
		DeviceID   string  `json:"device_id"`
		DeviceName string  `json:"device_name"`
		Value      float64 `json:"value"`
		Ts         int64   `json:"ts"`
		Status     int     `json:"status"`
	}

	type TagData struct {
		TagKey   string        `json:"tag_key"`
		TagName  string        `json:"tag_name"`
		Unit     string        `json:"unit"`
		DataType string        `json:"data_type"`
		Writable bool          `json:"writable"`
		Devices  []DeviceValue `json:"devices"`
	}

	result := make([]TagData, 0, len(projTags))
	for _, pt := range projTags {
		td := TagData{
			TagKey:   pt.TagKey,
			TagName:  pt.TagName,
			Unit:     pt.Unit,
			DataType: pt.DataType,
			Writable: pt.Writable,
			Devices:  make([]DeviceValue, 0),
		}

		for _, d := range devices {
			if _, ok := deviceTagMap[d.ID][pt.TagKey]; !ok {
				continue
			}
			dv := DeviceValue{
				DeviceID:   d.ID,
				DeviceName: d.Name,
				Status:     d.Status,
			}
			latest, err := h.store.GetLatestDataForDevice(d.ID)
			if err == nil {
				var parsed map[string]float64
				json.Unmarshal([]byte(latest.Data), &parsed)
				if v, ok := parsed[pt.TagKey]; ok {
					dv.Value = v
				}
				dv.Ts = latest.Ts
			}
			td.Devices = append(td.Devices, dv)
		}
		result = append(result, td)
	}

	success(c, gin.H{
		"project_id": pid,
		"tags":       result,
	})
}
