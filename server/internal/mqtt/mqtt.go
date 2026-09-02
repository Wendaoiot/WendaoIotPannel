package mqtt

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"

	"wendaoiotpannel/internal/evaluate"
	"wendaoiotpannel/internal/model"
	"wendaoiotpannel/internal/protocol"
	"wendaoiotpannel/internal/store"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	client mqtt.Client
	store  *store.Store
	cfg    Config
	mu     sync.RWMutex
}

func New(cfg Config, s *store.Store) (*Client, error) {
	c := &Client{store: s, cfg: cfg}

	opts := mqtt.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(cfg.ClientID).
		SetAutoReconnect(true).
		SetOnConnectHandler(func(_ mqtt.Client) {
			c.resubscribe()
		})

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}

	c.client = mqtt.NewClient(opts)
	return c, nil
}

func (c *Client) Connect() error {
	token := c.client.Connect()
	token.Wait()
	return token.Error()
}

func (c *Client) Subscribe() error {
	topics := map[string]byte{
		protocol.TopicDataSub():        1,
		protocol.TopicControlAckSub():  1,
		protocol.TopicOTAProgressSub(): 1,
		protocol.TopicOTAAckSub():      1,
	}
	token := c.client.SubscribeMultiple(topics, c.onMessage)
	token.Wait()
	return token.Error()
}

func (c *Client) resubscribe() {
	topics := map[string]byte{
		protocol.TopicDataSub():        1,
		protocol.TopicControlAckSub():  1,
		protocol.TopicOTAProgressSub(): 1,
		protocol.TopicOTAAckSub():      1,
	}
	c.client.SubscribeMultiple(topics, c.onMessage)
}

func extractDeviceID(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

func (c *Client) onMessage(_ mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	deviceID := extractDeviceID(topic)

	if strings.HasSuffix(topic, "/ota/progress") {
		c.handleOTAProgress(deviceID, msg.Payload())
	} else if strings.HasSuffix(topic, "/ota/ack") {
		c.handleOTAAck(deviceID, msg.Payload())
	} else if strings.HasSuffix(topic, "/data") {
		c.handleUplink(deviceID, msg.Payload())
	} else if strings.HasSuffix(topic, "/control/ack") {
		c.handleControlAck(msg.Payload())
	}
}

func (c *Client) handleUplink(deviceID string, payload []byte) {
	var req protocol.UplinkRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return
	}

	var resp protocol.UplinkResponse
	resp.ID = req.ID

	dev, err := c.store.GetDevice(deviceID)
	if err != nil {
		resp.Code = protocol.CodeDeviceNotRegistered
		resp.Msg = "device not registered"
		c.publishAck(protocol.TopicDataAck(deviceID), resp)
		return
	}

	if dev.Status == model.DeviceStatusInactive {
		resp.Code = protocol.CodeDeviceNotRegistered
		resp.Msg = "device inactive"
		c.publishAck(protocol.TopicDataAck(deviceID), resp)
		return
	}

	c.store.UpdateDeviceStatus(deviceID, model.DeviceStatusOnline)
	c.store.UpdateDeviceLastActive(deviceID)

	// Save first boot time (updated on each device reboot)
	if req.FirstTs > 0 {
		c.store.UpdateDeviceFirstTs(deviceID, req.FirstTs)
	}

	deviceTags, _ := c.store.ListDeviceTags(deviceID)
	tagMap := make(map[string]model.DeviceTag)
	for _, dt := range deviceTags {
		tagMap[dt.TagKey] = dt
	}

	computed := make(map[string]float64)
	for k, v := range req.Tags {
		computed[k] = v
		if dt, ok := tagMap[k]; ok && dt.Formula != "" {
			if cv, err := evaluate.Eval(dt.Formula, v); err == nil {
				computed[k] = cv
			}
		}
	}

	dataJSON, _ := json.Marshal(computed)
	dd := &model.DeviceData{
		DeviceID: deviceID,
		MsgID:    req.ID,
		Ts:       req.Ts,
		Version:  req.Version,
		Data:     string(dataJSON),
	}
	if err := c.store.SaveDeviceData(dd); err != nil {
	}

	resp.Code = protocol.CodeSuccess
	resp.Msg = "ok"
	c.publishAck(protocol.TopicDataAck(deviceID), resp)
}

func (c *Client) handleControlAck(payload []byte) {
	var ack protocol.DownlinkResponse
	if err := json.Unmarshal(payload, &ack); err != nil {
		return
	}
	c.store.UpdateControlLogAck(ack.ID, ack.Code, ack.Msg)
}

func (c *Client) publishAck(topic string, resp interface{}) {
	payload, _ := json.Marshal(resp)
	c.client.Publish(topic, 1, false, payload)
}

func (c *Client) PublishControl(deviceID string, cmd *protocol.DownlinkRequest) error {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	token := c.client.Publish(protocol.TopicControl(deviceID), 1, false, payload)
	token.Wait()
	return token.Error()
}

func (c *Client) PublishRaw(topic string, payload []byte) error {
	token := c.client.Publish(topic, 1, false, payload)
	token.Wait()
	return token.Error()
}

func (c *Client) handleOTAProgress(deviceID string, payload []byte) {
	var prog protocol.OTAProgress
	if err := json.Unmarshal(payload, &prog); err != nil {
		return
	}

	tasks, _ := c.store.ListOTATasks()
	for _, task := range tasks {
		if task.Status != model.OTATaskStatusRunning {
			continue
		}
		logs, _ := c.store.ListOTALogs(task.ID)
		for _, l := range logs {
			if l.DeviceID == deviceID && (l.Status == model.OTALogStatusPending || l.Status == model.OTALogStatusDownloading || l.Status == model.OTALogStatusInstalling) {
				c.store.UpdateOTALogStatus(deviceID, task.ID, prog.Status, prog.Progress, "")
				break
			}
		}
	}
}

func (c *Client) handleOTAAck(deviceID string, payload []byte) {
	var ack protocol.OTAResponse
	if err := json.Unmarshal(payload, &ack); err != nil {
		return
	}

	status := model.OTALogStatusSuccess
	errMsg := ""
	if ack.Code != 0 {
		status = model.OTALogStatusFailed
		errMsg = ack.Msg
	}

	tasks, _ := c.store.ListOTATasks()
	for _, task := range tasks {
		if task.Status != model.OTATaskStatusRunning {
			continue
		}
		logs, _ := c.store.ListOTALogs(task.ID)
		for _, l := range logs {
			if l.DeviceID == deviceID && (l.Status == model.OTALogStatusPending || l.Status == model.OTALogStatusDownloading || l.Status == model.OTALogStatusInstalling) {
				c.store.UpdateOTALogStatus(deviceID, task.ID, status, 100, errMsg)
				break
			}
		}
		c.store.CheckAndCompleteOTATask(task.ID)
	}
}

func StringFromUint(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}

type Config struct {
	Broker   string
	ClientID string
	Username string
	Password string
}
