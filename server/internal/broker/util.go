package broker

import (
	"encoding/json"
	"time"

	"wendaoiotpannel/internal/events"
	"wendaoiotpannel/internal/protocol"
)

// eventsMessage 构造前端广播消息（避免 broker 直接依赖 events.Message 的字段拼装散落）。
func eventsMessage(typ string, data map[string]interface{}) events.Message {
	return events.Message{Type: typ, Data: data}
}

// marshalKickedNotice 序列化被踢通知（协议结构见 protocol.KickedNotice）。
func marshalKickedNotice(reason, msg string) ([]byte, error) {
	return json.Marshal(protocol.KickedNotice{Reason: reason, Msg: msg, Ts: time.Now().UnixMilli()})
}
