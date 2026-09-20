// Package domain 提供易支付协议的签名算法、金额换算与凭证生成。
package domain

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// 业务常量
const (
	RoleUser  int64 = 0
	RoleAdmin int64 = 1

	OrderStatusPending int64 = 0
	OrderStatusPaid    int64 = 1
	OrderStatusRefund  int64 = 2
	OrderStatusClosed  int64 = 3

	SettleStatusPending int64 = 0
	SettleStatusPaid    int64 = 1
	SettleStatusReject  int64 = 2

	BalanceLogOrderIncome int64 = 0
	BalanceLogWithdraw    int64 = 1
	BalanceLogWithdrawRet int64 = 2
	BalanceLogAdminAdjust int64 = 3
	BalanceLogRefund      int64 = 4
)

// Channels 支持的支付通道。
var Channels = []string{"alipay", "wxpay", "qqpay"}

// ValidChannel 校验通道合法。
func ValidChannel(c string) bool {
	for _, ch := range Channels {
		if ch == c {
			return true
		}
	}
	return false
}

// ChannelName 通道中文名。
func ChannelName(c string) string {
	switch c {
	case "alipay":
		return "支付宝"
	case "wxpay":
		return "微信支付"
	case "qqpay":
		return "QQ钱包"
	}
	return c
}

// Sign 易支付标准 MD5 签名：按参数名 ASCII 升序拼 k=v&…（忽略 sign/sign_type 与空值），末尾直接拼接商户密钥。
func Sign(params map[string]string, key string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		if b.Len() > 0 {
			b.WriteByte('&')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	b.WriteString(key)
	sum := md5.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// VerifySign 校验参数集合的签名是否正确（sign_type 须为 MD5）。
func VerifySign(params map[string]string, key, sign string) bool {
	if params["sign_type"] != "MD5" {
		return false
	}
	return sign != "" && Sign(params, key) == sign
}

// FenToYuan 分转元字符串，如 199 -> "1.99"。
func FenToYuan(fen int64) string {
	sign := ""
	if fen < 0 {
		sign = "-"
		fen = -fen
	}
	return fmt.Sprintf("%s%d.%02d", sign, fen/100, fen%100)
}

// YuanToFen 元字符串转分，最多两位小数；非法返回 error。
func YuanToFen(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("金额不能为空")
	}
	neg := false
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("金额不能为负数")
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("金额格式错误")
	}
	yuan, err := parseDigits(parts[0])
	if err != nil {
		return 0, err
	}
	frac := int64(0)
	if len(parts) == 2 {
		f := parts[1]
		if len(f) > 2 {
			return 0, fmt.Errorf("金额最多两位小数")
		}
		for i := 0; i < 2; i++ {
			frac *= 10
			if i < len(f) {
				d := int64(f[i] - '0')
				if d < 0 || d > 9 {
					return 0, fmt.Errorf("金额格式错误")
				}
				frac += d
			}
		}
	}
	if neg {
		return -(yuan*100 + frac), nil
	}
	return yuan*100 + frac, nil
}

func parseDigits(s string) (int64, error) {
	if s == "" {
		s = "0"
	}
	n := new(big.Int)
	if _, ok := n.SetString(s, 10); !ok {
		return 0, fmt.Errorf("金额格式错误")
	}
	if n.Cmp(big.NewInt(1_000_000_000)) > 0 {
		return 0, fmt.Errorf("金额过大")
	}
	return n.Int64(), nil
}

// GenTradeNo 生成不可枚举的平台订单号：F + 时间戳 + 16 位随机十六进制。
func GenTradeNo() string {
	random := GenKey()
	if len(random) > 16 {
		random = random[:16]
	}
	return "F" + time.Now().Format("20060102150405") + random
}

// GenKey 生成 32 位十六进制商户密钥 / 会话令牌。
func GenKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// HashPassword 生成 bcrypt 散列。
func HashPassword(raw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword 校验明文与散列是否匹配。
func CheckPassword(hash, raw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}

// AnyI64 将 SQLite 聚合结果（interface{} 承载的整型）归一为 int64。
func AnyI64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case uint64:
		return int64(n)
	default:
		return 0
	}
}
