package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type WeChatNotifier struct {
	corpID  string
	agentID int64
	secret  string

	httpClient *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpire time.Time
}

func NewWeChatNotifier(corpID string, agentID int64, secret string) *WeChatNotifier {
	return &WeChatNotifier{
		corpID:     corpID,
		agentID:    agentID,
		secret:     secret,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// 发送文本消息
func (w *WeChatNotifier) SendText(toUserID, content string) error {
	return w.SendTextContext(context.Background(), toUserID, content)
}

func (w *WeChatNotifier) SendTextContext(ctx context.Context, toUserID, content string) error {
	if strings.TrimSpace(toUserID) == "" {
		return errors.New("wechat: missing toUserID")
	}
	if strings.TrimSpace(content) == "" {
		return errors.New("wechat: missing content")
	}
	content = truncateWeChatText(content, 1800)

	msg := wechatSendMessageRequest{
		ToUser:  toUserID,
		MsgType: "text",
		AgentID: w.agentID,
		Text: &wechatTextMessage{
			Content: content,
		},
	}
	return w.sendMessage(ctx, msg)
}

func truncateWeChatText(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	rs := []rune(s)
	if len(rs) <= maxRunes {
		return s
	}
	// 预留提示尾巴
	tail := "\n\n（内容过长已截断，如需更多请继续提问或缩小范围）"
	tailRunes := []rune(tail)
	limit := maxRunes
	if maxRunes > len(tailRunes)+50 {
		limit = maxRunes - len(tailRunes)
	}
	out := string(rs[:limit]) + tail
	return out
}

func (w *WeChatNotifier) getAccessToken(ctx context.Context) (string, error) {
	now := time.Now()

	w.mu.Lock()
	if w.accessToken != "" && now.Before(w.tokenExpire) {
		token := w.accessToken
		w.mu.Unlock()
		return token, nil
	}
	w.mu.Unlock()

	if strings.TrimSpace(w.corpID) == "" || strings.TrimSpace(w.secret) == "" {
		return "", errors.New("wechat: missing corpID or secret")
	}

	u := url.URL{
		Scheme: "https",
		Host:   "qyapi.weixin.qq.com",
		Path:   "/cgi-bin/gettoken",
	}
	q := u.Query()
	q.Set("corpid", w.corpID)
	q.Set("corpsecret", w.secret)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("wechat: gettoken http %d: %s", resp.StatusCode, string(body))
	}

	var out wechatGetTokenResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("wechat: gettoken decode: %w", err)
	}
	if out.ErrCode != 0 {
		return "", fmt.Errorf("wechat: gettoken errcode=%d errmsg=%s", out.ErrCode, out.ErrMsg)
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		return "", errors.New("wechat: gettoken returned empty access_token")
	}

	// 预留 60s 缓冲，避免临界时间 token 失效
	expireAt := now.Add(time.Duration(out.ExpiresIn)*time.Second - 60*time.Second)

	w.mu.Lock()
	w.accessToken = out.AccessToken
	w.tokenExpire = expireAt
	w.mu.Unlock()

	return out.AccessToken, nil
}

func (w *WeChatNotifier) sendMessage(ctx context.Context, msg wechatSendMessageRequest) error {
	token, err := w.getAccessToken(ctx)
	if err != nil {
		return err
	}

	u := url.URL{
		Scheme: "https",
		Host:   "qyapi.weixin.qq.com",
		Path:   "/cgi-bin/message/send",
	}
	q := u.Query()
	q.Set("access_token", token)
	u.RawQuery = q.Encode()

	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("wechat: send http %d: %s", resp.StatusCode, string(body))
	}

	var out wechatSendMessageResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("wechat: send decode: %w", err)
	}
	if out.ErrCode != 0 {
		return fmt.Errorf("wechat: send errcode=%d errmsg=%s invaliduser=%s", out.ErrCode, out.ErrMsg, out.InvalidUser)
	}
	return nil
}

type wechatGetTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type wechatSendMessageRequest struct {
	ToUser  string `json:"touser,omitempty"`
	MsgType string `json:"msgtype"`
	AgentID int64  `json:"agentid"`

	Text *wechatTextMessage `json:"text,omitempty"`
}

type wechatTextMessage struct {
	Content string `json:"content"`
}

type wechatSendMessageResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	InvalidUser string `json:"invaliduser,omitempty"`
}
