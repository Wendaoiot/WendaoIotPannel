package store

import (
	"strconv"
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

func (s *Store) CreateTenant(t *model.Tenant) error {
	return s.db.Create(t).Error
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
		var projectIDs []uint
		if r := tx.Model(&model.Project{}).Where("tenant_id = ?", id).Pluck("id", &projectIDs); r.Error != nil {
			return r.Error
		}
		if len(projectIDs) > 0 {
			var deviceIDs []string
			if r := tx.Model(&model.Device{}).Where("project_id IN ?", projectIDs).Pluck("id", &deviceIDs); r.Error != nil {
				return r.Error
			}
			if len(deviceIDs) > 0 {
				tx.Where("device_id IN ?", deviceIDs).Delete(&model.DeviceTag{})
				tx.Where("device_id IN ?", deviceIDs).Delete(&model.DeviceData{})
				tx.Where("device_id IN ?", deviceIDs).Delete(&model.ControlLog{})
				tx.Where("device_id IN ?", deviceIDs).Delete(&model.OTALog{})
				tx.Where("project_id IN ?", projectIDs).Delete(&model.Device{})
			}
			tx.Where("project_id IN ?", projectIDs).Delete(&model.ProjectTag{})
			tx.Where("target_type = ? AND target_id IN ?", "project", uintsToStrings(projectIDs)).Delete(&model.OTATask{})
			tx.Where("project_id IN ?", projectIDs).Delete(&model.Project{})
		}
		tx.Where("tenant_id = ?", id).Delete(&model.AdminUser{})
		return tx.Delete(&model.Tenant{}, id).Error
	})
}

// Project

func (s *Store) CreateProject(p *model.Project) error {
	return s.db.Create(p).Error
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
		if r := tx.Model(&model.Device{}).Where("project_id = ?", id).Pluck("id", &deviceIDs); r.Error != nil {
			return r.Error
		}
		if len(deviceIDs) > 0 {
			tx.Where("device_id IN ?", deviceIDs).Delete(&model.DeviceTag{})
			tx.Where("device_id IN ?", deviceIDs).Delete(&model.DeviceData{})
			tx.Where("device_id IN ?", deviceIDs).Delete(&model.ControlLog{})
			tx.Where("device_id IN ?", deviceIDs).Delete(&model.OTALog{})
			tx.Where("project_id = ?", id).Delete(&model.Device{})
		}
		tx.Where("project_id = ?", id).Delete(&model.ProjectTag{})
		tx.Where("target_type = ? AND target_id = ?", "project", strconv.FormatUint(uint64(id), 10)).Delete(&model.OTATask{})
		return tx.Delete(&model.Project{}, id).Error
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

func (s *Store) UpdateDevice(deviceID string, name string, projectID uint, status int) error {
	return s.db.Model(&model.Device{}).Where("id = ?", deviceID).Updates(map[string]interface{}{
		"name":       name,
		"project_id": projectID,
		"status":     status,
	}).Error
}

func (s *Store) DeleteDevice(deviceID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		tx.Where("device_id = ?", deviceID).Delete(&model.DeviceTag{})
		tx.Where("device_id = ?", deviceID).Delete(&model.DeviceData{})
		tx.Where("device_id = ?", deviceID).Delete(&model.ControlLog{})
		tx.Where("device_id = ?", deviceID).Delete(&model.OTALog{})
		return tx.Delete(&model.Device{}, "id = ?", deviceID).Error
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

// MarkOfflineDevices marks devices as offline if they haven't been active for the specified duration
func (s *Store) MarkOfflineDevices(timeout time.Duration) error {
	return s.db.Model(&model.Device{}).
		Where("status = ? AND last_active < ?", model.DeviceStatusOnline, time.Now().Add(-timeout)).
		Update("status", model.DeviceStatusOffline).Error
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
