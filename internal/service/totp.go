package service

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	totpPeriod = 30
	totpDigits = 6
)

// totpCode 计算某个时间点对应的 TOTP 动态口令。
func totpCode(secret string, t time.Time) (string, error) {
	key, err := base32Decode(secret)
	if err != nil {
		return "", err
	}
	counter := uint64(t.Unix()) / totpPeriod
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	code := (binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff) % 1_000_000
	return fmt.Sprintf("%0*d", totpDigits, code), nil
}

func base32Decode(s string) ([]byte, error) {
	s = strings.TrimRight(strings.ToUpper(s), "=")
	padded := s + strings.Repeat("=", (8-len(s)%8)%8)
	return base32.StdEncoding.DecodeString(padded)
}

// VerifyTOTP 校验动态口令，允许前后各 1 个时间窗口容差。
func VerifyTOTP(secret, code string, t time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}
	for _, offset := range []int{-1, 0, 1} {
		tt := t.Add(time.Duration(offset) * totpPeriod * time.Second)
		expect, err := totpCode(secret, tt)
		if err != nil {
			return false
		}
		if hmac.Equal([]byte(expect), []byte(code)) {
			return true
		}
	}
	return false
}

// TOTPSetupURL 生成用于扫码绑定 TOTP 客户端的 otpauth:// URL。
func TOTPSetupURL(secret, issuer, account string) string {
	params := url.Values{}
	params.Set("secret", secret)
	params.Set("issuer", issuer)
	params.Set("algorithm", "SHA1")
	params.Set("digits", fmt.Sprintf("%d", totpDigits))
	params.Set("period", fmt.Sprintf("%d", totpPeriod))
	return fmt.Sprintf("otpauth://totp/%s?%s", url.PathEscape(issuer+":"+account), params.Encode())
}
