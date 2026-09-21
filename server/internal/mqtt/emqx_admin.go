package mqtt

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// EMQX 5.x REST API 客户端：用于"同设备重复连接互踢"。
// 认证方式：Authorization: <ApiKey>:<ApiSecret> 的 Base64（EMQX Dashboard → API Keys）。

type EMQXAdmin struct {
	base   string
	apiKey string
	secret string
	http   *http.Client
}

func NewEMQXAdmin(apiBase, apiKey, apiSecret string) *EMQXAdmin {
	return &EMQXAdmin{
		base:   strings.TrimRight(apiBase, "/"),
		apiKey: apiKey,
		secret: apiSecret,
		http:   &http.Client{Timeout: 5 * time.Second},
	}
}

// Enabled 判断互踢依赖的 EMQX API 配置是否完整。
func (a *EMQXAdmin) Enabled() bool {
	return a != nil && a.base != "" && a.apiKey != "" && a.secret != ""
}

type emqxClientItem struct {
	ClientID string `json:"clientid"`
	Username string `json:"username"`
	// proto_ver 在 EMQX 5.8 响应里是数字（3/4/5），用 json.Number 兼容数字/字符串两种形态
	Protocol json.Number `json:"proto_ver"`
}

type emqxListResp struct {
	Data []emqxClientItem `json:"data"`
}

// listClientsByUsername 拉取某 username（=设备ID）当前的全部在线连接。
func (a *EMQXAdmin) listClientsByUsername(username string) ([]emqxClientItem, error) {
	u := fmt.Sprintf("%s/api/v5/clients?username=%s&limit=100", a.base, url.QueryEscape(username))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", emqxAuthHeader(a.apiKey, a.secret))
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("emqx list clients: HTTP %d: %.200s", resp.StatusCode, body)
	}
	var out emqxListResp
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// KickDeviceSessions 踢掉某设备（username）当前的在线会话，返回被踢的 clientid 列表。
// excludeClientID 为新连接自身的 clientid：异步执行时新连接可能已注册，必须排除以免踢到自己；
// 传空则踢掉全部（禁用/reactivate 场景）。
// 被踢连接会触发 EMQX disconnected 系统事件，平台在线状态由现有 $SYS 逻辑自动回收。
func (a *EMQXAdmin) KickDeviceSessions(username, excludeClientID string) ([]string, error) {
	if !a.Enabled() {
		return nil, nil
	}
	clients, err := a.listClientsByUsername(username)
	if err != nil {
		return nil, err
	}
	var kicked []string
	for _, cl := range clients {
		if cl.ClientID == "" || cl.ClientID == excludeClientID {
			continue
		}
		if err := a.deleteClient(cl.ClientID); err != nil {
			log.Printf("emqx admin: kick clientid=%s (device=%s) failed: %v", cl.ClientID, username, err)
			continue
		}
		kicked = append(kicked, cl.ClientID)
	}
	return kicked, nil
}

func (a *EMQXAdmin) deleteClient(clientID string) error {
	u := fmt.Sprintf("%s/api/v5/clients/%s", a.base, url.PathEscape(clientID))
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", emqxAuthHeader(a.apiKey, a.secret))
	resp, err := a.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	// 404 视为成功（连接恰好已断开）
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("emqx kick: HTTP %d", resp.StatusCode)
	}
	return nil
}

func emqxAuthHeader(key, secret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(key+":"+secret))
}
