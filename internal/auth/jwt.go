package auth

import (
	"time" // 需要 time

	"github.com/golang-jwt/jwt/v5" // 需要 jwt
)

// !!! 重要的安全提示 !!!
// 在真实应用中，绝不要把密钥硬编码在代码里！
// 应该从环境变量或安全配置中读取。
var JwtKey = []byte("my_super_secret_signing_key_123!@#")

type Claims struct {
	Username string `json:"username"`
	UserID   int    `json:"user_id"`
	Role     string `json:"role"` // <-- 新增 Role 字段
	jwt.RegisteredClaims
}

// GenerateJWT 生成一个新的 JWT 字符串
func GenerateJWT(userID int, username string, role string) (string, time.Time, error) { // <-- 添加 role 参数 {
    // 设置过期时间 (例如，1 小时)
	expirationTime := time.Now().Add(24 * time.Hour)

	claims := &Claims{
		Username: username,
		UserID:   userID,
		Role:     role, // <-- 将 role 加入 Claims
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Subject:   username, // 可以用 UserID 的字符串形式或 Username
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(JwtKey) // 使用导出的 JwtKey

	return tokenString, expirationTime, err // 同时返回 token 字符串、过期时间和错误
}
