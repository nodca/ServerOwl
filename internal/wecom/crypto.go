package wecom

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

type Crypto struct {
	Token string
	Key   []byte // 32 bytes AES key
	CorpID string
}

func NewCrypto(token, encodingAESKey, corpID string) (*Crypto, error) {
	if token == "" {
		return nil, errors.New("wecom: missing token")
	}
	if corpID == "" {
		return nil, errors.New("wecom: missing corpID")
	}
	if encodingAESKey == "" {
		return nil, errors.New("wecom: missing encodingAESKey")
	}
	// 企业微信 EncodingAESKey 通常为 43 位 base64，无 '='
	raw, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, fmt.Errorf("wecom: decode encodingAESKey: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("wecom: invalid AES key length: %d", len(raw))
	}
	return &Crypto{
		Token:  token,
		Key:    raw,
		CorpID: corpID,
	}, nil
}

func (c *Crypto) MsgSignature(timestamp, nonce, encrypted string) string {
	parts := []string{c.Token, timestamp, nonce, encrypted}
	sort.Strings(parts)
	h := sha1.Sum([]byte(parts[0] + parts[1] + parts[2] + parts[3]))
	return fmt.Sprintf("%x", h)
}

func (c *Crypto) VerifyMsgSignature(msgSignature, timestamp, nonce, encrypted string) bool {
	if msgSignature == "" || timestamp == "" || nonce == "" || encrypted == "" {
		return false
	}
	return c.MsgSignature(timestamp, nonce, encrypted) == msgSignature
}

func (c *Crypto) Decrypt(encrypted string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("wecom: base64 decode encrypt: %w", err)
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("wecom: invalid ciphertext length")
	}

	block, err := aes.NewCipher(c.Key)
	if err != nil {
		return nil, fmt.Errorf("wecom: new cipher: %w", err)
	}
	iv := c.Key[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	plain := make([]byte, len(ciphertext))
	mode.CryptBlocks(plain, ciphertext)

	// 16 bytes random + 4 bytes msgLen + msg + corpID + padding
	// 企业微信的填充可能超过16字节，所以不使用PKCS7解填充，直接根据msgLen提取消息
	if len(plain) < 20 {
		return nil, errors.New("wecom: plaintext too short")
	}
	msgLen := binary.BigEndian.Uint32(plain[16:20])
	msgStart := 20
	msgEnd := msgStart + int(msgLen)
	if msgEnd > len(plain) {
		return nil, errors.New("wecom: invalid msg length")
	}
	msg := plain[msgStart:msgEnd]

	// 提取 corpID：从消息结束到填充开始
	// 填充字节的值等于填充长度，从末尾向前找到非填充字节
	padLen := int(plain[len(plain)-1])
	if padLen > len(plain)-msgEnd {
		padLen = 0 // 无效填充，忽略
	}
	corpIDEnd := len(plain) - padLen
	if corpIDEnd <= msgEnd {
		return nil, errors.New("wecom: invalid corpID position")
	}
	appID := plain[msgEnd:corpIDEnd]
	if !bytes.Equal(appID, []byte(c.CorpID)) {
		return nil, fmt.Errorf("wecom: corpID mismatch: got %q, want %q", string(appID), c.CorpID)
	}
	return msg, nil
}

func pkcs7Unpad(b []byte, blockSize int) ([]byte, error) {
	if len(b) == 0 || len(b)%blockSize != 0 {
		return nil, errors.New("wecom: invalid padding size")
	}
	pad := int(b[len(b)-1])
	if pad == 0 || pad > blockSize || pad > len(b) {
		return nil, errors.New("wecom: invalid padding")
	}
	for i := len(b) - pad; i < len(b); i++ {
		if int(b[i]) != pad {
			return nil, errors.New("wecom: invalid padding")
		}
	}
	return b[:len(b)-pad], nil
}

