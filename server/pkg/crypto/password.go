// Package crypto 提供与业务无关的密码哈希与随机口令工具。
// 密码统一使用 bcrypt（自带盐、慢哈希），替代历史的裸 SHA-256。
package crypto

import (
	"crypto/rand"
	"errors"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

// MinPasswordLength 已弃用：系统按要求不限制密码强度，仅保留常量供兼容。
const MinPasswordLength = 0

// bcryptCost 控制 bcrypt 工作因子；10 是兼顾安全与性能的常用值。
const bcryptCost = 10

// passwordAlphabet 去除了易混淆字符（0/O、1/l/I）的随机口令字母表。
const passwordAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// ErrPasswordTooShort 已弃用（不再使用）。
var ErrPasswordTooShort = errors.New("密码长度不能少于8位")

// HashPassword 使用 bcrypt 生成加盐哈希。
func HashPassword(pwd string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pwd), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// MustHashPassword 用于启动期种子等“失败即不可继续”的场景。
func MustHashPassword(pwd string) string {
	h, err := HashPassword(pwd)
	if err != nil {
		panic(err)
	}
	return h
}

// CheckPassword 校验明文密码是否匹配 bcrypt 哈希。
func CheckPassword(hashed, pwd string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(pwd)) == nil
}

// PasswordPolicyDisabled 系统按要求不限制管理员密码强度（仅要求非空）。
// 保留空函数供历史调用方兼容；新增代码请勿再调用。
func ValidatePasswordStrength(pwd string) error {
	if len([]rune(pwd)) == 0 {
		return errors.New("密码不能为空")
	}
	return nil
}

// RandomPassword 生成指定长度的随机临时口令（用于未显式指定密码时）。
func RandomPassword(n int) (string, error) {
	if n <= 0 {
		n = 12
	}
	out := make([]byte, n)
	for i := range out {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordAlphabet))))
		if err != nil {
			return "", err
		}
		out[i] = passwordAlphabet[idx.Int64()]
	}
	return string(out), nil
}
