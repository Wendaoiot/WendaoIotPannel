package store

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"wendaoiotpannel/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func New(dsn string) (*Store, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) AutoMigrate() error {
	return s.db.AutoMigrate(
		&model.Tenant{},
		&model.Project{},
		&model.Device{},
		&model.DeviceTag{},
		&model.ProjectTag{},
		&model.DeviceData{},
		&model.ControlLog{},
		&model.AdminUser{},
		&model.Firmware{},
		&model.OTATask{},
		&model.OTALog{},
		&model.ControlCommand{},
		&model.DeviceMessage{},
		&model.DevicePeerAllow{},
	)
}

// AdminUser

func (s *Store) GetAdminUserByUsername(username string) (*model.AdminUser, error) {
	var u model.AdminUser
	err := s.db.First(&u, "username = ?", username).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CreateAdminUser(u *model.AdminUser) error {
	return s.db.Create(u).Error
}

func (s *Store) ListAdminUsers(tenantID *uint) ([]model.AdminUser, error) {
	var users []model.AdminUser
	q := s.db.Omit("password")
	if tenantID != nil {
		q = q.Where("tenant_id = ?", *tenantID)
	}
	err := q.Order("id").Find(&users).Error
	return users, err
}

func (s *Store) GetAdminUserByID(id uint) (*model.AdminUser, error) {
	var u model.AdminUser
	err := s.db.Omit("password").First(&u, id).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) UpdateAdminUserPassword(id uint, hashedPassword string) error {
	return s.db.Model(&model.AdminUser{}).Where("id = ?", id).Update("password", hashedPassword).Error
}

func (s *Store) DeleteAdminUser(id uint) error {
	return s.db.Delete(&model.AdminUser{}, id).Error
}

// Tenant

// CreateTenant 创建租户，ID 取最低空闲位：删除后新建可复用低位 ID（如全删后从 1 重新开始）。
func (s *Store) CreateTenant(t *model.Tenant) error {
	for attempt := 0; attempt < 3; attempt++ {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var ids []uint
			if err := tx.Unscoped().Model(&model.Tenant{}).Pluck("id", &ids).Error; err != nil {
				return err
			}
			used := make(map[uint]bool, len(ids))
			for _, id := range ids {
				used[id] = true
			}
			next := uint(1)
			for used[next] {
				next++
			}
			t.ID = next
			return tx.Create(t).Error
		})
		if err == nil {
			return nil
		}
		// 并发抢占同一最低位：重算后重试
		if !strings.Contains(err.Error(), "1062") && !strings.Contains(err.Error(), "Duplicate") {
			return err
		}
	}
	return errors.New("创建租户失败：ID 分配并发冲突，请重试")
}

// TenantNameExists 检查租户名是否已被活跃租户占用（只看未删行，软删残留不算）。
func (s *Store) TenantNameExists(name string) (bool, error) {
	var n int64
	err := s.db.Model(&model.Tenant{}).Where("name = ?", name).Count(&n).Error
	return n > 0, err
}

func (s *Store) ListTenants() ([]model.Tenant, error) {
	var tenants []model.Tenant
	err := s.db.Find(&tenants).Error
	return tenants, err
}

func (s *Store) GetTenantByID(id uint) (*model.Tenant, error) {
	var t model.Tenant
	err := s.db.First(&t, id).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) UpdateTenant(id uint, name string) error {
	return s.db.Model(&model.Tenant{}).Where("id = ?", id).Update("name", name).Error
}

func (s *Store) DeleteTenant(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 物理删除（Unscoped）：租户及子树全部清除，不留软删残留，
		// 名称与 ID 随之释放（配合 CreateTenant 最低位分配实现复用）。
		var projectIDs []uint
		if r := tx.Unscoped().Model(&model.Project{}).Where("tenant_id = ?", id).Pluck("id", &projectIDs); r.Error != nil {
			return r.Error
		}
		if len(projectIDs) > 0 {
			var deviceIDs []string
			if r := tx.Unscoped().Model(&model.Device{}).Where("project_id IN ?", projectIDs).Pluck("id", &deviceIDs); r.Error != nil {
				return r.Error
			}
			if len(deviceIDs) > 0 {
				if r := tx.Unscoped().Where("device_id IN ?", deviceIDs).Delete(&model.DeviceTag{}); r.Error != nil {
					return r.Error
				}
				if r := tx.Unscoped().Where("device_id IN ?", deviceIDs).Delete(&model.DeviceData{}); r.Error != nil {
					return r.Error
				}
				if r := tx.Unscoped().Where("device_id IN ?", deviceIDs).Delete(&model.ControlLog{}); r.Error != nil {
					return r.Error
				}
				if r := tx.Unscoped().Where("device_id IN ?", deviceIDs).Delete(&model.OTALog{}); r.Error != nil {
					return r.Error
				}
				// 设备间消息：任一端（发/收）命中即清
				if r := tx.Unscoped().Where("from_device_id IN ? OR to_device_id IN ?", deviceIDs, deviceIDs).Delete(&model.DeviceMessage{}); r.Error != nil {
					return r.Error
				}
				if r := tx.Unscoped().Where("from_device_id IN ? OR to_device_id IN ?", deviceIDs, deviceIDs).Delete(&model.DevicePeerAllow{}); r.Error != nil {
					return r.Error
				}
				if r := tx.Unscoped().Where("id IN ?", deviceIDs).Delete(&model.Device{}); r.Error != nil {
					return r.Error
				}
			}
			// 项目维度子表：自定义控制按钮、项目标签
			if r := tx.Unscoped().Where("project_id IN ?", projectIDs).Delete(&model.ControlCommand{}); r.Error != nil {
				return r.Error
			}
			if r := tx.Unscoped().Where("project_id IN ?", projectIDs).Delete(&model.ProjectTag{}); r.Error != nil {
				return r.Error
			}
			if r := tx.Unscoped().Where("target_type = ? AND target_id IN ?", "project", uintsToStrings(projectIDs)).Delete(&model.OTATask{}); r.Error != nil {
				return r.Error
			}
			// 删除项目：条件必须是主键 id IN ?（此前误写 project_id IN ? 导致 1054 未知列错误被静默吞掉，项目全部残留）
			if r := tx.Unscoped().Where("id IN ?", projectIDs).Delete(&model.Project{}); r.Error != nil {
				return r.Error
			}
		}
		if r := tx.Unscoped().Where("tenant_id = ?", id).Delete(&model.AdminUser{}); r.Error != nil {
			return r.Error
		}
		if r := tx.Unscoped().Delete(&model.Tenant{}, id); r.Error != nil {
			return r.Error
		}
		return nil
	})
}

// Project

// CreateProject 创建项目，ID 取最低空闲位（与 CreateTenant 同策略）：
// 删除项目后新建可复用低位 ID（如全删后从 1 重新开始）。并发抢占同一最低位时重试。
func (s *Store) CreateProject(p *model.Project) error {
	for attempt := 0; attempt < 3; attempt++ {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			var ids []uint
			if err := tx.Unscoped().Model(&model.Project{}).Pluck("id", &ids).Error; err != nil {
				return err
			}
			used := make(map[uint]bool, len(ids))
			for _, id := range ids {
				used[id] = true
			}
			next := uint(1)
			for used[next] {
				next++
			}
			p.ID = next
			return tx.Create(p).Error
		})
		if err == nil {
			return nil
		}
		// 并发抢占同一最低位：重算后重试
		if !strings.Contains(err.Error(), "1062") && !strings.Contains(err.Error(), "Duplicate") {
			return err
		}
	}
	return errors.New("创建项目失败：ID 分配并发冲突，请重试")
}

func (s *Store) GetProjectByID(id uint) (*model.Project, error) {
	var p model.Project
	err := s.db.First(&p, id).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) ListProjectsByTenant(tenantID uint) ([]model.Project, error) {
	var projects []model.Project
	err := s.db.Where("tenant_id = ?", tenantID).Find(&projects).Error
	return projects, err
}

func (s *Store) ListAllProjects() ([]model.Project, error) {
	var projects []model.Project
	err := s.db.Find(&projects).Error
	return projects, err
}

func (s *Store) UpdateProject(id uint, name string) error {
	return s.db.Model(&model.Project{}).Where("id = ?", id).Update("name", name).Error
}

func (s *Store) DeleteProject(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var deviceIDs []string
		if r := tx.Unscoped().Model(&model.Device{}).Where("project_id = ?", id).Pluck("id", &deviceIDs); r.Error != nil {
			return r.Error
		}
		if len(deviceIDs) > 0 {
			if r := tx.Unscoped().Where("device_id IN ?", deviceIDs).Delete(&model.DeviceTag{}); r.Error != nil {
				return r.Error
			}
			if r := tx.Unscoped().Where("device_id IN ?", deviceIDs).Delete(&model.DeviceData{}); r.Error != nil {
				return r.Error
			}
			if r := tx.Unscoped().Where("device_id IN ?", deviceIDs).Delete(&model.ControlLog{}); r.Error != nil {
				return r.Error
			}
			if r := tx.Unscoped().Where("device_id IN ?", deviceIDs).Delete(&model.OTALog{}); r.Error != nil {
				return r.Error
			}
			// 设备间消息：任一端（发/收）命中即清
			if r := tx.Unscoped().Where("from_device_id IN ? OR to_device_id IN ?", deviceIDs, deviceIDs).Delete(&model.DeviceMessage{}); r.Error != nil {
				return r.Error
			}
			if r := tx.Unscoped().Where("from_device_id IN ? OR to_device_id IN ?", deviceIDs, deviceIDs).Delete(&model.DevicePeerAllow{}); r.Error != nil {
				return r.Error
			}
			if r := tx.Unscoped().Where("id IN ?", deviceIDs).Delete(&model.Device{}); r.Error != nil {
				return r.Error
			}
		}
		// 项目维度子表：自定义控制按钮、项目标签
		if r := tx.Unscoped().Where("project_id = ?", id).Delete(&model.ControlCommand{}); r.Error != nil {
			return r.Error
		}
		if r := tx.Unscoped().Where("project_id = ?", id).Delete(&model.ProjectTag{}); r.Error != nil {
			return r.Error
		}
		if r := tx.Unscoped().Where("target_type = ? AND target_id = ?", "project", strconv.FormatUint(uint64(id), 10)).Delete(&model.OTATask{}); r.Error != nil {
			return r.Error
		}
		return tx.Unscoped().Delete(&model.Project{}, id).Error
	})
}

// Device

func (s *Store) CreateDevice(d *model.Device) error {
	return s.db.Create(d).Error
}

func (s *Store) GetDevice(deviceID string) (*model.Device, error) {
	var d model.Device
	err := s.db.First(&d, "id = ?", deviceID).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) ListDevicesByProject(projectID uint) ([]model.Device, error) {
	var devices []model.Device
	err := s.db.Where("project_id = ?", projectID).Find(&devices).Error
	return devices, err
}

func (s *Store) ListDevicesByProjects(projectIDs []uint) ([]model.Device, error) {
	var devices []model.Device
	if len(projectIDs) == 0 {
		err := s.db.Find(&devices).Error
		return devices, err
	}
	err := s.db.Where("project_id IN ?", projectIDs).Find(&devices).Error
	return devices, err
}

func (s *Store) UpdateDeviceStatus(deviceID string, status int) error {
	return s.db.Model(&model.Device{}).Where("id = ?", deviceID).Update("status", status).Error
}

// UpdateDevice 更新设备可编辑属性（名称/所属项目/在线判定配置）。
// 注意：status 为运行时状态（MQTT 事件/超时扫描维护），绝不允许由此接口写入。
func (s *Store) UpdateDevice(deviceID string, name string, projectID uint, onlineMode string, offlineTimeoutSec int) error {
	return s.db.Model(&model.Device{}).Where("id = ?", deviceID).Updates(map[string]interface{}{
		"name":                name,
		"project_id":          projectID,
		"online_mode":         onlineMode,
		"offline_timeout_sec": offlineTimeoutSec,
	}).Error
}

// DeleteDevice 物理删除设备及其全部从属数据（Unscoped）。
// 设备主键即用户填写的字符串 ID，软删除会残留行导致同 ID 重建撞主键 1062，
// 因此与删租户/项目一致采用物理删除，删除后该 ID 立即可复用。每步均检查错误，杜绝静默吞错。
func (s *Store) DeleteDevice(deviceID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if r := tx.Unscoped().Where("device_id = ?", deviceID).Delete(&model.DeviceTag{}); r.Error != nil {
			return r.Error
		}
		if r := tx.Unscoped().Where("device_id = ?", deviceID).Delete(&model.DeviceData{}); r.Error != nil {
			return r.Error
		}
		if r := tx.Unscoped().Where("device_id = ?", deviceID).Delete(&model.ControlLog{}); r.Error != nil {
			return r.Error
		}
		if r := tx.Unscoped().Where("device_id = ?", deviceID).Delete(&model.OTALog{}); r.Error != nil {
			return r.Error
		}
		// 设备间消息/授权：任一端（发/收）命中即清
		if r := tx.Unscoped().Where("from_device_id = ? OR to_device_id = ?", deviceID, deviceID).Delete(&model.DeviceMessage{}); r.Error != nil {
			return r.Error
		}
		if r := tx.Unscoped().Where("from_device_id = ? OR to_device_id = ?", deviceID, deviceID).Delete(&model.DevicePeerAllow{}); r.Error != nil {
			return r.Error
		}
		// 设备维度的自定义控制指令（device_id 留空的项目通用指令不在此列）
		if r := tx.Unscoped().Where("device_id = ?", deviceID).Delete(&model.ControlCommand{}); r.Error != nil {
			return r.Error
		}
		// 设备维度的 OTA 任务
		if r := tx.Unscoped().Where("target_type = ? AND target_id = ?", "device", deviceID).Delete(&model.OTATask{}); r.Error != nil {
			return r.Error
		}
		return tx.Unscoped().Delete(&model.Device{}, "id = ?", deviceID).Error
	})
}

// DeviceTag

func (s *Store) CreateDeviceTag(t *model.DeviceTag) error {
	return s.db.Create(t).Error
}

func (s *Store) ListDeviceTags(deviceID string) ([]model.DeviceTag, error) {
	var tags []model.DeviceTag
	err := s.db.Where("device_id = ?", deviceID).Find(&tags).Error
	return tags, err
}

func (s *Store) DeleteDeviceTag(id uint) error {
	return s.db.Delete(&model.DeviceTag{}, id).Error
}

// ProjectTag

func (s *Store) CreateProjectTag(t *model.ProjectTag) error {
	return s.db.Create(t).Error
}

func (s *Store) ListProjectTags(projectID uint) ([]model.ProjectTag, error) {
	var tags []model.ProjectTag
	err := s.db.Where("project_id = ?", projectID).Find(&tags).Error
	return tags, err
}

func (s *Store) DeleteProjectTag(id uint) error {
	return s.db.Delete(&model.ProjectTag{}, id).Error
}

// DeviceData

func (s *Store) SaveDeviceData(d *model.DeviceData) error {
	return s.db.Create(d).Error
}

func (s *Store) ListDeviceData(deviceID string, limit int, offset int) ([]model.DeviceData, int64, error) {
	var total int64
	s.db.Model(&model.DeviceData{}).Where("device_id = ?", deviceID).Count(&total)
	var data []model.DeviceData
	err := s.db.Where("device_id = ?", deviceID).Order("ts desc").Limit(limit).Offset(offset).Find(&data).Error
	return data, total, err
}

// ControlLog

func (s *Store) CreateControlLog(l *model.ControlLog) error {
	return s.db.Create(l).Error
}

func (s *Store) GetControlLogByMsgID(msgID string) (*model.ControlLog, error) {
	var l model.ControlLog
	err := s.db.First(&l, "msg_id = ?", msgID).Error
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Store) UpdateControlLogAck(msgID string, code int, msg string) error {
	return s.db.Model(&model.ControlLog{}).Where("msg_id = ?", msgID).Updates(map[string]interface{}{
		"ack_code": code,
		"ack_msg":  msg,
	}).Error
}

func (s *Store) ListControlLogs(deviceID string, limit int, offset int) ([]model.ControlLog, int64, error) {
	var total int64
	q := s.db.Model(&model.ControlLog{})
	if deviceID != "" {
		q = q.Where("device_id = ?", deviceID)
	}
	q.Count(&total)
	var logs []model.ControlLog
	err := q.Order("id desc").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

// Stats

type DashboardStats struct {
	TotalTenants  int64 `json:"total_tenants"`
	TotalProjects int64 `json:"total_projects"`
	TotalDevices  int64 `json:"total_devices"`
	OnlineDevices int64 `json:"online_devices"`
}

func (s *Store) GetDashboardStats(tenantID *uint) (*DashboardStats, error) {
	var stats DashboardStats
	if tenantID == nil {
		s.db.Model(&model.Tenant{}).Count(&stats.TotalTenants)
		s.db.Model(&model.Project{}).Count(&stats.TotalProjects)
		s.db.Model(&model.Device{}).Count(&stats.TotalDevices)
		s.db.Model(&model.Device{}).Where("status = ?", model.DeviceStatusOnline).Count(&stats.OnlineDevices)
	} else {
		var projectIDs []uint
		s.db.Model(&model.Project{}).Where("tenant_id = ?", *tenantID).Pluck("id", &projectIDs)
		stats.TotalProjects = int64(len(projectIDs))
		s.db.Model(&model.Device{}).Where("project_id IN ?", projectIDs).Count(&stats.TotalDevices)
		s.db.Model(&model.Device{}).Where("project_id IN ? AND status = ?", projectIDs, model.DeviceStatusOnline).Count(&stats.OnlineDevices)
	}
	return &stats, nil
}

func (s *Store) ListDevicesByProjectIDs(projectIDs []uint) ([]model.Device, error) {
	if len(projectIDs) == 0 {
		return nil, nil
	}
	var devices []model.Device
	err := s.db.Where("project_id IN ?", projectIDs).Find(&devices).Error
	return devices, err
}

func (s *Store) GetLatestDataForDevice(deviceID string) (*model.DeviceData, error) {
	var d model.DeviceData
	err := s.db.Where("device_id = ?", deviceID).Order("ts desc").First(&d).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// Firmware

func (s *Store) CreateFirmware(fw *model.Firmware) error {
	return s.db.Create(fw).Error
}

func (s *Store) ListFirmwares() ([]model.Firmware, error) {
	var list []model.Firmware
	err := s.db.Order("id desc").Find(&list).Error
	return list, err
}

func (s *Store) GetFirmwareByID(id uint) (*model.Firmware, error) {
	var fw model.Firmware
	err := s.db.First(&fw, id).Error
	if err != nil {
		return nil, err
	}
	return &fw, nil
}

func (s *Store) GetFirmwareByVersion(version string) (*model.Firmware, error) {
	var fw model.Firmware
	err := s.db.Where("version = ?", version).First(&fw).Error
	if err != nil {
		return nil, err
	}
	return &fw, nil
}

func (s *Store) GetLatestFirmware() (*model.Firmware, error) {
	var fw model.Firmware
	err := s.db.Order("id desc").First(&fw).Error
	if err != nil {
		return nil, err
	}
	return &fw, nil
}

func (s *Store) DeleteFirmware(id uint) error {
	return s.db.Delete(&model.Firmware{}, id).Error
}

// OTATask

func (s *Store) CreateOTATask(t *model.OTATask) error {
	return s.db.Create(t).Error
}

func (s *Store) ListOTATasks() ([]model.OTATask, error) {
	var tasks []model.OTATask
	err := s.db.Order("id desc").Find(&tasks).Error
	return tasks, err
}

func (s *Store) UpdateOTATaskStatus(taskID uint, status string) error {
	return s.db.Model(&model.OTATask{}).Where("id = ?", taskID).Update("status", status).Error
}

func (s *Store) CheckAndCompleteOTATask(taskID uint) {
	var logs []model.OTALog
	s.db.Where("task_id = ?", taskID).Find(&logs)
	if len(logs) == 0 {
		return
	}
	allDone := true
	for _, l := range logs {
		if l.Status != model.OTALogStatusSuccess && l.Status != model.OTALogStatusFailed {
			allDone = false
			break
		}
	}
	if allDone {
		s.UpdateOTATaskStatus(taskID, model.OTATaskStatusDone)
	}
}

// OTALog

func (s *Store) CreateOTALog(log *model.OTALog) error {
	return s.db.Create(log).Error
}

func (s *Store) ListOTALogs(taskID uint) ([]model.OTALog, error) {
	var logs []model.OTALog
	err := s.db.Where("task_id = ?", taskID).Order("id").Find(&logs).Error
	return logs, err
}

func (s *Store) UpdateOTALogStatus(deviceID string, taskID uint, status string, progress int, errMsg string) error {
	return s.db.Model(&model.OTALog{}).
		Where("device_id = ? AND task_id = ?", deviceID, taskID).
		Updates(map[string]interface{}{
			"status":    status,
			"progress":  progress,
			"error_msg": errMsg,
		}).Error
}

func uintsToStrings(ids []uint) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = strconv.FormatUint(uint64(id), 10)
	}
	return result
}

// MarkOfflineDevices 离线回收扫描（设备级在线判定感知）。
// 有效模式 = COALESCE(NULLIF(online_mode, ”), 'connection')：历史空串一律按默认 connection 处理。
//   - report：超过有效超时未上报(last_active 陈旧)即判离线；
//   - ping ：超过有效超时未回应 ping(last_active 陈旧)即判离线（连接假死也能检出）；
//   - connection：纯事件驱动（断开即离线），扫描不触碰。
//
// 有效超时 = 设备 offline_timeout_sec(>0) 优先，否则全局 timeout（全局<=0 视为禁用超时，不回收）。
func (s *Store) MarkOfflineDevices(timeout time.Duration) error {
	now := time.Now()
	effMode := "COALESCE(NULLIF(online_mode, ''), 'connection')"

	// ① 自定义超时的 report/ping 设备：按各自 offline_timeout_sec 回收
	if err := s.db.Model(&model.Device{}).
		Where("status = ? AND "+effMode+" IN ('report','ping') AND offline_timeout_sec > 0 AND last_active < DATE_SUB(?, INTERVAL offline_timeout_sec SECOND)",
			model.DeviceStatusOnline, now).
		Update("status", model.DeviceStatusOffline).Error; err != nil {
		return err
	}

	// ② 未自定义超时的 report/ping 设备：沿用全局超时（全局禁用超时时跳过）
	if timeout > 0 {
		if err := s.db.Model(&model.Device{}).
			Where("status = ? AND "+effMode+" IN ('report','ping') AND offline_timeout_sec <= 0 AND last_active < ?",
				model.DeviceStatusOnline, now.Add(-timeout)).
			Update("status", model.DeviceStatusOffline).Error; err != nil {
			return err
		}
	}
	return nil
}

// ListPingModeDevices 返回有效判定模式为 ping 且已启用的设备 ID 列表（平台探活发送目标）。
// 包含当前离线设备：ping 到达且设备应答后可经 handlePingAck 恢复在线。
func (s *Store) ListPingModeDevices() ([]string, error) {
	var ids []string
	err := s.db.Model(&model.Device{}).
		Where("enabled = ? AND COALESCE(NULLIF(online_mode, ''), 'connection') = ?", true, "ping").
		Pluck("id", &ids).Error
	return ids, err
}

// UpdateDeviceLastActive updates the device's last active time
func (s *Store) UpdateDeviceLastActive(deviceID string) error {
	return s.db.Model(&model.Device{}).
		Where("id = ?", deviceID).
		Update("last_active", time.Now()).Error
}

// UpdateDeviceFirstTs updates the device's first boot time
func (s *Store) UpdateDeviceFirstTs(deviceID string, firstTs int64) error {
	return s.db.Model(&model.Device{}).
		Where("id = ?", deviceID).
		Update("first_ts", firstTs).Error
}
