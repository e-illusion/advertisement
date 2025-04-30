package middleware

import (
	"context"
	"errors" // 需要导入 errors 包来处理 jwt v5 的错误
	"fmt"
	"log"
	"net/http"
	"strings"
	//"time" // 可能需要 time 来处理 jwt.ErrTokenNotValidYet

	"github.com/golang-jwt/jwt/v5"

	// --- 导入内部包 ---
	"advertisement/internal/auth"   // 替换 "your_module_name"
	"advertisement/internal/webutil" // 替换 "your_module_name"
)

// contextKey 是用于 context 的 key 的类型
type contextKey string

// UserContextKey 是用于在 context 中存储用户 Claims 的键
const UserContextKey contextKey = "user"

// AuthMiddleware 是检查 JWT 的认证中间件
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			webutil.RespondWithError(w, http.StatusUnauthorized, "缺少认证 Token") // 使用 webutil
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			webutil.RespondWithError(w, http.StatusUnauthorized, "认证 Token 格式错误 (应为 'Bearer <token>')") // 使用 webutil
			return
		}
		tokenString := parts[1]

		claims := &auth.Claims{} // 使用 auth.Claims
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("非预期的签名算法: %v", token.Header["alg"])
			}
			return auth.JwtKey, nil // 使用 auth.JwtKey
		})

		if err != nil {
			// 使用 errors.Is 来检查包装过的错误 (jwt v5 推荐)
			if errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet) {
   				// 在 v5 中，过期和尚未生效的错误可能都归类到 ErrTokenNotValidYet 或 ErrTokenExpired
   				webutil.RespondWithError(w, http.StatusUnauthorized, "Token 已过期或无效")
			} else if errors.Is(err, jwt.ErrSignatureInvalid) {
                webutil.RespondWithError(w, http.StatusUnauthorized, "无效的 Token 签名")
            } else {
				log.Printf("Token 解析错误: %v", err)
				webutil.RespondWithError(w, http.StatusUnauthorized, "无效的 Token") // 不要透露过多错误细节
			}
			return
		}

		if !token.Valid {
			webutil.RespondWithError(w, http.StatusUnauthorized, "无效的 Token") // 使用 webutil
			return
		}

		// Token 有效，将 Claims 存入 context
		// 注意这里用的是包内定义的 contextKey 类型和导出的 UserContextKey 常量
		ctx := context.WithValue(r.Context(), UserContextKey, claims)

		// 使用带有新 context 的请求副本调用下一个 handler
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// --- 新增：AdminMiddleware 检查用户是否为管理员 ---
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 假设此中间件在 AuthMiddleware 之后执行
		// 从 Context 获取用户信息
		userClaims, ok := r.Context().Value(UserContextKey).(*auth.Claims)
		if !ok || userClaims == nil {
			// 如果 AuthMiddleware 没有设置 Claims，或者类型不匹配，这是服务器内部错误
			log.Println("错误：AdminMiddleware 无法从 context 获取有效的用户信息")
			webutil.RespondWithError(w, http.StatusInternalServerError, "无法验证用户权限")
			return
		}

		// 检查用户角色
		if userClaims.Role != "admin" {
			log.Printf("权限不足：用户 %s (ID: %d, Role: %s) 尝试访问管理员接口", userClaims.Username, userClaims.UserID, userClaims.Role)
			webutil.RespondWithError(w, http.StatusForbidden, "权限不足，需要管理员权限") // 返回 403 Forbidden
			return
		}

		// 用户是管理员，调用下一个 handler
		log.Printf("管理员访问: %s (ID: %d)", userClaims.Username, userClaims.UserID)
		next.ServeHTTP(w, r)
	})
}
