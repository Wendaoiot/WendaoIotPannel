package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

type UplinkRequest struct {
	ID   string             `json:"id"`
	Ts   int64              `json:"ts"`
	Tags map[string]float64 `json:"tags"`
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
	Status   string `json:"status"`
}

type OTAResponse struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type tagConfig struct {
	Key  string
	Min  float64
	Max  float64
	Dest int
}

var (
	deviceID    string
	broker      string
	interval    int
	tagsConfig  string
	mqttUser    string
	mqttPass    string
)

func main() {
	flag.StringVar(&deviceID, "id", "ESP32-001", "设备ID (MAC/SN)")
	flag.StringVar(&broker, "broker", "tcp://127.0.0.1:1883", "MQTT Broker地址")
	flag.IntVar(&interval, "interval", 5, "数据上报间隔(秒)")
	flag.StringVar(&tagsConfig, "tags", "temp:20:35:1,humidity:40:80:1,voltage:3.1:3.3:2", "标签配置 格式: key:min:max:decimals,...")
	flag.StringVar(&mqttUser, "user", "", "MQTT用户名")
	flag.StringVar(&mqttPass, "pass", "", "MQTT密码")
	flag.Parse()

	log.SetPrefix(fmt.Sprintf("[%s] ", deviceID))
	log.SetFlags(log.Ltime)

	tags := parseTags(tagsConfig)
	log.Printf("标签配置: %s", tagsConfig)
	log.Printf("上报间隔: %ds", interval)

	clientID := fmt.Sprintf("sim_%s", deviceID)
	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(clientID).
		SetAutoReconnect(true).
		SetKeepAlive(30 * time.Second).
		SetPingTimeout(10 * time.Second)

	if mqttUser != "" {
		opts.SetUsername(mqttUser)
		opts.SetPassword(mqttPass)
	}

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Println("已连接 MQTT Broker")
		controlTopic := fmt.Sprintf("wendao/%s/control", deviceID)
		c.Subscribe(controlTopic, 1, onControl)

		otaTopic := fmt.Sprintf("wendao/%s/ota", deviceID)
		c.Subscribe(otaTopic, 1, onOTA)
		log.Printf("已订阅: %s, %s", controlTopic, otaTopic)
	})

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if token.Error() != nil {
		log.Fatalf("连接 MQTT Broker 失败: %v", token.Error())
	}

	go dataLoop(client, tags)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("模拟器退出")
	client.Disconnect(250)
}

func parseTags(cfg string) []tagConfig {
	var tags []tagConfig
	for _, item := range strings.Split(cfg, ",") {
		parts := strings.Split(strings.TrimSpace(item), ":")
		if len(parts) < 3 {
			continue
		}
		t := tagConfig{Key: parts[0]}
		fmt.Sscanf(parts[1], "%f", &t.Min)
		fmt.Sscanf(parts[2], "%f", &t.Max)
		if len(parts) > 3 {
			fmt.Sscanf(parts[3], "%d", &t.Dest)
		}
		tags = append(tags, t)
	}
	if len(tags) == 0 {
		tags = []tagConfig{
			{Key: "temp", Min: 20, Max: 35, Dest: 1},
			{Key: "humidity", Min: 40, Max: 80, Dest: 1},
			{Key: "voltage", Min: 3.1, Max: 3.3, Dest: 2},
		}
	}
	return tags
}

func dataLoop(client mqtt.Client, tags []tagConfig) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		msgID := uuid.New().String()
		tagValues := make(map[string]float64, len(tags))
		for _, t := range tags {
			tagValues[t.Key] = round(t.Min+rand.Float64()*(t.Max-t.Min), t.Dest)
		}

		req := UplinkRequest{
			ID:   msgID,
			Ts:   time.Now().UnixMilli(),
			Tags: tagValues,
		}
		payload, _ := json.Marshal(req)

		topic := fmt.Sprintf("wendao/%s/data", deviceID)
		token := client.Publish(topic, 1, false, payload)
		token.Wait()
		if token.Error() != nil {
			log.Printf("上报数据失败: %v", token.Error())
		} else {
			var parts []string
			for _, t := range tags {
				parts = append(parts, fmt.Sprintf("%s=%.*f", t.Key, t.Dest, tagValues[t.Key]))
			}
			log.Printf("上报数据: %s (id=%s)", strings.Join(parts, " "), msgID[:8])
		}
	}
}

func onControl(client mqtt.Client, msg mqtt.Message) {
	var cmd DownlinkRequest
	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		log.Printf("解析控制指令失败: %v", err)
		return
	}

	tagList := make([]string, 0, len(cmd.Tags))
	for k, v := range cmd.Tags {
		tagList = append(tagList, fmt.Sprintf("%s=%v", k, v))
	}
	log.Printf("收到控制指令: [%s] (id=%s)", strings.Join(tagList, ", "), cmd.ID[:8])

	time.Sleep(200 * time.Millisecond)

	ack := DownlinkResponse{
		ID:   cmd.ID,
		Code: 0,
		Msg:  "ok",
	}
	ackPayload, _ := json.Marshal(ack)
	topic := fmt.Sprintf("wendao/%s/control/ack", deviceID)
	ackToken := client.Publish(topic, 1, false, ackPayload)
	ackToken.Wait()
	if ackToken.Error() != nil {
		log.Printf("回复控制ACK失败: %v", ackToken.Error())
	} else {
		log.Printf("已回复控制ACK: id=%s code=0", cmd.ID[:8])
	}
}

func onOTA(client mqtt.Client, msg mqtt.Message) {
	var ota OTARequest
	if err := json.Unmarshal(msg.Payload(), &ota); err != nil {
		log.Printf("解析OTA指令失败: %v", err)
		return
	}
	log.Printf("收到OTA指令: version=%s url=%s size=%d (id=%s)", ota.Version, ota.URL, ota.Size, ota.ID[:8])

	for _, pct := range []int{10, 30, 60, 90} {
		time.Sleep(800 * time.Millisecond)
		prog := OTAProgress{
			ID:       ota.ID,
			Type:     "ota_progress",
			Progress: pct,
			Status:   "downloading",
		}
		progPayload, _ := json.Marshal(prog)
		topic := fmt.Sprintf("wendao/%s/ota/progress", deviceID)
		client.Publish(topic, 1, false, progPayload)
		log.Printf("OTA进度: %d%%", pct)
	}

	time.Sleep(1 * time.Second)
	instProg := OTAProgress{
		ID:       ota.ID,
		Type:     "ota_progress",
		Progress: 100,
		Status:   "installing",
	}
	instPayload, _ := json.Marshal(instProg)
	topic := fmt.Sprintf("wendao/%s/ota/progress", deviceID)
	client.Publish(topic, 1, false, instPayload)
	log.Printf("OTA进度: 100%% installing")

	time.Sleep(1 * time.Second)
	ack := OTAResponse{
		ID:   ota.ID,
		Type: "ota_ack",
		Code: 0,
		Msg:  "success",
	}
	ackPayload, _ := json.Marshal(ack)
	ackTopic := fmt.Sprintf("wendao/%s/ota/ack", deviceID)
	client.Publish(ackTopic, 1, false, ackPayload)
	log.Printf("OTA升级完成: version=%s", ota.Version)
}

func round(v float64, decimals int) float64 {
	pow := 1.0
	for i := 0; i < decimals; i++ {
		pow *= 10
	}
	return float64(int(v*pow+0.5)) / pow
}
