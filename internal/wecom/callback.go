package wecom

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"
)

type CallbackHandler struct {
	Crypto *Crypto

	// OnText 在解密并解析出文本消息后触发；用于把指令交给你的业务处理。
	OnText func(fromUserID, content string)

	// Logf 用于输出调试日志（可选）。建议传入 log.Printf / logger.Printf。
	Logf func(format string, args ...any)

	// 消息去重
	processedMsgs sync.Map // map[string]time.Time，存储已处理的 MsgId
}

func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Crypto == nil {
		if h != nil && h.Logf != nil {
			h.Logf("wecom: missing crypto")
		}
		http.Error(w, "wecom: missing crypto", http.StatusInternalServerError)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleVerify(w, r)
	case http.MethodPost:
		h.handleMessage(w, r)
	default:
		if h.Logf != nil {
			h.Logf("wecom: method not allowed method=%s path=%s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *CallbackHandler) handleVerify(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	msgSig := q.Get("msg_signature")
	timestamp := q.Get("timestamp")
	nonce := q.Get("nonce")
	echostr := q.Get("echostr")

	if !h.Crypto.VerifyMsgSignature(msgSig, timestamp, nonce, echostr) {
		if h.Logf != nil {
			h.Logf("wecom: verify failed (url verify) path=%s", r.URL.Path)
		}
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	plain, err := h.Crypto.Decrypt(echostr)
	if err != nil {
		if h.Logf != nil {
			h.Logf("wecom: decrypt echostr failed err=%v", err)
		}
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(plain)
}

func (h *CallbackHandler) handleMessage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	msgSig := q.Get("msg_signature")
	timestamp := q.Get("timestamp")
	nonce := q.Get("nonce")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		if h.Logf != nil {
			h.Logf("wecom: read body failed err=%v remote=%s", err, r.RemoteAddr)
		}
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}
	var env encryptedEnvelope
	if err := xml.Unmarshal(body, &env); err != nil {
		if h.Logf != nil {
			h.Logf("wecom: invalid envelope xml err=%v remote=%s", err, r.RemoteAddr)
		}
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if env.Encrypt == "" {
		if h.Logf != nil {
			h.Logf("wecom: missing Encrypt field remote=%s", r.RemoteAddr)
		}
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !h.Crypto.VerifyMsgSignature(msgSig, timestamp, nonce, env.Encrypt) {
		if h.Logf != nil {
			h.Logf("wecom: verify failed (message) path=%s remote=%s", r.URL.Path, r.RemoteAddr)
		}
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	plain, err := h.Crypto.Decrypt(env.Encrypt)
	if err != nil {
		if h.Logf != nil {
			h.Logf(
				"wecom: decrypt failed err=%v remote=%s xff=%s ua=%q ts=%s nonce=%s encrypt_len=%d",
				err,
				r.RemoteAddr,
				r.Header.Get("X-Forwarded-For"),
				r.Header.Get("User-Agent"),
				timestamp,
				nonce,
				len(env.Encrypt),
			)
		}
		// 不暴露“签名通过/解密失败”的差异，避免作为 token 探测/爆破的 oracle。
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var msg inboundMessage
	if err := xml.Unmarshal(plain, &msg); err != nil {
		if h.Logf != nil {
			h.Logf("wecom: invalid message xml err=%v", err)
		}
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if h.Logf != nil {
		h.Logf(
			"wecom: inbound msgtype=%s from=%s remote=%s xff=%s ua=%q",
			msg.MsgType,
			msg.FromUserName,
			r.RemoteAddr,
			r.Header.Get("X-Forwarded-For"),
			r.Header.Get("User-Agent"),
		)
	}
	if msg.MsgType == "text" && h.OnText != nil {
		// 消息去重：检查 MsgId 是否已处理
		if msg.MsgId != "" {
			if _, exists := h.processedMsgs.LoadOrStore(msg.MsgId, time.Now()); exists {
				if h.Logf != nil {
					h.Logf("wecom: duplicate message ignored msgid=%s", msg.MsgId)
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("success"))
				return
			}

			// 清理 5 分钟前的记录（防止内存泄漏）
			go h.cleanupOldMessages()
		}

		h.OnText(msg.FromUserName, msg.Content)
	}

	// 这里先用最简单的“成功响应”，避免超时。业务回复建议走主动发送接口(message/send)。
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("success"))
}

type encryptedEnvelope struct {
	XMLName xml.Name `xml:"xml"`
	Encrypt string   `xml:"Encrypt"`
}

type inboundMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgId        string   `xml:"MsgId"` // 用于消息去重
}

// 清理 5 分钟前的消息记录
func (h *CallbackHandler) cleanupOldMessages() {
	cutoff := time.Now().Add(-5 * time.Minute)
	h.processedMsgs.Range(func(key, value any) bool {
		if t, ok := value.(time.Time); ok && t.Before(cutoff) {
			h.processedMsgs.Delete(key)
		}
		return true
	})
}

var ErrNotConfigured = errors.New("wecom: callback not configured")
