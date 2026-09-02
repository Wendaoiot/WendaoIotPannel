package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"wendaoiotpannel/internal/model"
	"wendaoiotpannel/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("wendaoiot-secret-key")

func InitJWTSecret(secret string) {
	if secret != "" {
		jwtSecret = []byte(secret)
	}
}

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	TenantID *uint  `json:"tenant_id"`
	jwt.RegisteredClaims
}

func hashPassword(pwd string) string {
	h := sha256.Sum256([]byte(pwd))
	return hex.EncodeToString(h[:])
}

func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		Role     string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}

	user, err := h.store.GetAdminUserByUsername(req.Username)
	if err != nil {
		fail(c, 401, "username or password incorrect")
		return
	}

	if user.Role != req.Role {
		fail(c, 401, "role mismatch")
		return
	}

	if user.Password != hashPassword(req.Password) {
		fail(c, 401, "username or password incorrect")
		return
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		TenantID: user.TenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}).SignedString(jwtSecret)
	if err != nil {
		fail(c, -1, err.Error())
		return
	}

	success(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":        user.ID,
			"username":  user.Username,
			"role":      user.Role,
			"tenant_id": user.TenantID,
		},
	})
}

func AuthMiddleware(s *store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Response{Code: 401, Msg: "unauthorized"})
			return
		}

		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Response{Code: 401, Msg: "invalid token"})
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, Response{Code: 401, Msg: "invalid token"})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("tenant_id", claims.TenantID)
		c.Next()
	}
}

func SeedAdminUsers(s *store.Store) {
	superAdmin := model.AdminUser{
		Username: "admin",
		Password: hashPassword("admin123"),
		Role:     model.RoleSuperAdmin,
	}
	if _, err := s.GetAdminUserByUsername(superAdmin.Username); err != nil {
		s.CreateAdminUser(&superAdmin)
	}
}

// User management handlers

func (h *Handler) ListUsers(c *gin.Context) {
	role, tenantID := getAuthInfo(c)
	if role != model.RoleSuperAdmin {
		tenantID = getTenantID(c)
	}
	users, err := h.store.ListAdminUsers(tenantID)
	if err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, users)
}

func (h *Handler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		fail(c, 401, "unauthorized")
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}

	uid := userID.(uint)
	user, err := h.store.GetAdminUserByID(uid)
	if err != nil {
		fail(c, -1, "用户不存在")
		return
	}

	userFull, err := h.store.GetAdminUserByUsername(user.Username)
	if err != nil {
		fail(c, -1, "用户不存在")
		return
	}

	if userFull.Password != hashPassword(req.OldPassword) {
		fail(c, -1, "原密码不正确")
		return
	}

	if len(req.NewPassword) < 4 {
		fail(c, -1, "新密码长度不能少于4位")
		return
	}

	if err := h.store.UpdateAdminUserPassword(uid, hashPassword(req.NewPassword)); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, nil)
}

func (h *Handler) AdminResetUserPassword(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		fail(c, -1, "invalid id")
		return
	}
	var req struct {
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, -1, err.Error())
		return
	}
	if len(req.NewPassword) < 4 {
		fail(c, -1, "密码长度不能少于4位")
		return
	}
	if err := h.store.UpdateAdminUserPassword(uint(id), hashPassword(req.NewPassword)); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, nil)
}

func (h *Handler) DeleteUser(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		fail(c, -1, "invalid id")
		return
	}
	userID, _ := c.Get("user_id")
	if uint(id) == userID.(uint) {
		fail(c, -1, "不能删除自己")
		return
	}
	if err := h.store.DeleteAdminUser(uint(id)); err != nil {
		fail(c, -1, err.Error())
		return
	}
	success(c, nil)
}

func getTenantID(c *gin.Context) *uint {
	val, exists := c.Get("tenant_id")
	if !exists || val == nil {
		return nil
	}
	if tid, ok := val.(*uint); ok {
		return tid
	}
	return nil
}

func parseUintParam(c *gin.Context, param string) (uint64, error) {
	s := c.Param(param)
	var n uint64
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}
