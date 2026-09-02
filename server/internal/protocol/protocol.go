package protocol

const (
	CodeSuccess             = 0
	CodeDeviceNotRegistered = 1
	CodeTagNotConfigured    = 2
	CodeParamError          = 3
)

type UplinkRequest struct {
	ID      string             `json:"id"`
	Ts      int64              `json:"ts"`
	Version string             `json:"version"`
	FirstTs int64              `json:"first_ts"`
	Tags    map[string]float64 `json:"tags"`
}

type UplinkResponse struct {
	ID   string `json:"id"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type DownlinkRequest struct {
	ID   string             `json:"id"`
	Ts   int64              `json:"ts"`
	Tags map[string]float64 `json:"tags"`
}

type DownlinkResponse struct {
	ID   string `json:"id"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func TopicData(deviceID string) string        { return "wendao/" + deviceID + "/data" }
func TopicDataAck(deviceID string) string     { return "wendao/" + deviceID + "/data/ack" }
func TopicControl(deviceID string) string     { return "wendao/" + deviceID + "/control" }
func TopicControlAck(deviceID string) string  { return "wendao/" + deviceID + "/control/ack" }
func TopicDataSub() string                    { return "wendao/+/data" }
func TopicControlAckSub() string              { return "wendao/+/control/ack" }
func TopicOTA(deviceID string) string         { return "wendao/" + deviceID + "/ota" }
func TopicOTAProgress(deviceID string) string { return "wendao/" + deviceID + "/ota/progress" }
func TopicOTAAck(deviceID string) string      { return "wendao/" + deviceID + "/ota/ack" }
func TopicOTAProgressSub() string             { return "wendao/+/ota/progress" }
func TopicOTAAckSub() string                  { return "wendao/+/ota/ack" }

type OTARequest struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Version string `json:"version"`
	URL     string `json:"url"`
	MD5     string `json:"md5"`
	Size    int64  `json:"size"`
}

type OTAProgress struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Progress int    `json:"progress"`
	Status   string `json:"status"` // downloading/installing
}

type OTAResponse struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
