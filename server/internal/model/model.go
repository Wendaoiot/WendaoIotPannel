package model

import "time"

type Tenant struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(100);uniqueIndex" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Project struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index" json:"tenant_id"`
	Name      string    `gorm:"type:varchar(100)" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

const (
	DeviceStatusOffline  = 0
	DeviceStatusOnline   = 1
	DeviceStatusInactive = 2
)

type Device struct {
	ID         string     `gorm:"primaryKey;type:varchar(100)" json:"id"`
	ProjectID  uint       `gorm:"index" json:"project_id"`
	Name       string     `gorm:"type:varchar(100)" json:"name"`
	Status     int        `gorm:"default:0" json:"status"`
	FirstTs    int64      `gorm:"default:0" json:"first_ts"`
	LastActive *time.Time `gorm:"index" json:"last_active"`
	CreatedAt  time.Time  `json:"created_at"`
}

type DeviceTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  string    `gorm:"index;type:varchar(100)" json:"device_id"`
	TagKey    string    `gorm:"type:varchar(50)" json:"tag_key"`
	Interface string    `gorm:"type:varchar(50)" json:"interface"`
	Formula   string    `gorm:"type:varchar(500)" json:"formula"`
	CreatedAt time.Time `json:"created_at"`
}

type ProjectTag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ProjectID uint      `gorm:"index" json:"project_id"`
	TagKey    string    `gorm:"type:varchar(50)" json:"tag_key"`
	TagName   string    `gorm:"type:varchar(100)" json:"tag_name"`
	Unit      string    `gorm:"type:varchar(20)" json:"unit"`
	DataType  string    `gorm:"type:varchar(20);default:'number'" json:"data_type"`
	Writable  bool      `gorm:"default:false" json:"writable"`
	CreatedAt time.Time `json:"created_at"`
}

type DeviceData struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  string    `gorm:"index;type:varchar(100)" json:"device_id"`
	MsgID     string    `gorm:"type:varchar(100)" json:"msg_id"`
	Ts        int64     `json:"ts"`
	Version   string    `gorm:"type:varchar(50)" json:"version"`
	Data      string    `gorm:"type:text" json:"data"`
	CreatedAt time.Time `json:"created_at"`
}

type ControlLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DeviceID  string    `gorm:"index;type:varchar(100)" json:"device_id"`
	MsgID     string    `gorm:"type:varchar(100)" json:"msg_id"`
	Tags      string    `gorm:"type:text" json:"tags"`
	AckCode   *int      `json:"ack_code"`
	AckMsg    string    `json:"ack_msg"`
	CreatedAt time.Time `json:"created_at"`
}

const (
	RoleSuperAdmin  = "super_admin"
	RoleTenantAdmin = "tenant_admin"
)

type AdminUser struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"type:varchar(50);uniqueIndex" json:"username"`
	Password  string    `gorm:"type:varchar(200)" json:"-"`
	Role      string    `gorm:"type:varchar(20)" json:"role"`
	TenantID  *uint     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Firmware struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"type:varchar(100)" json:"name"`
	Version     string    `gorm:"type:varchar(50);index" json:"version"`
	URL         string    `gorm:"type:varchar(500)" json:"url"`
	Size        int64     `json:"size"`
	MD5         string    `gorm:"type:varchar(64)" json:"md5"`
	Description string    `gorm:"type:varchar(500)" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

const (
	OTATaskStatusPending = "pending"
	OTATaskStatusRunning = "running"
	OTATaskStatusDone    = "done"
)

type OTATask struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	FirmwareID uint      `json:"firmware_id"`
	TargetType string    `gorm:"type:varchar(20)" json:"target_type"` // device/project
	TargetID   string    `gorm:"type:varchar(100)" json:"target_id"`
	Status     string    `gorm:"type:varchar(20);default:'pending'" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

const (
	OTALogStatusPending     = "pending"
	OTALogStatusDownloading = "downloading"
	OTALogStatusInstalling  = "installing"
	OTALogStatusSuccess     = "success"
	OTALogStatusFailed      = "failed"
)

type OTALog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    uint      `gorm:"index" json:"task_id"`
	DeviceID  string    `gorm:"type:varchar(100);index" json:"device_id"`
	Status    string    `gorm:"type:varchar(20);default:'pending'" json:"status"`
	Progress  int       `gorm:"default:0" json:"progress"`
	ErrorMsg  string    `gorm:"type:varchar(500)" json:"error_msg"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
