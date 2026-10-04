package userdomain

import (
	"crypto/rand"
	"math/big"
	"strings"
)

// inviteCodeAlphabet 邀请码字符表。
// 排除易混淆字符 0/O/1/I，与 affiliate 推广码保持一致的可读风格。
const inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// InviteCodeLength 邀请码长度（8 位）。
const InviteCodeLength = 8

// GenerateInviteCode 生成一个随机、不可猜测的个人邀请码。
// 使用 crypto/rand，大写字母+数字，排除 0/O/1/I。
// 调用方负责在写入前检查全局唯一性（碰撞则重试）。
func GenerateInviteCode() (string, error) {
	max := big.NewInt(int64(len(inviteCodeAlphabet)))
	var builder strings.Builder
	builder.Grow(InviteCodeLength)
	for i := 0; i < InviteCodeLength; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(inviteCodeAlphabet[n.Int64()])
	}
	return builder.String(), nil
}

// NormalizeInviteCode 归一化外部输入的邀请码（去空白、转大写）。
func NormalizeInviteCode(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}
