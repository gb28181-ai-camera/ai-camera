package auth

import (
	"errors"
	"time"

	"github.com/boticz/ai-camera-backend/config"
	"github.com/golang-jwt/jwt/v5"
)

// Claims 是签发到 JWT 中的用户上下文。
// 除身份标识外还携带数据权限三要素（data_perm / company_id / cross_dept_ids），
// 使后续请求无需再查库即可完成行级数据过滤。
type Claims struct {
	UserID       int64  `json:"user_id"`
	Username     string `json:"user_name"` // 登录名（user_name 列），非显示名
	RoleCode     string `json:"role_code"`
	DeptID       int64  `json:"dept_id"`
	DataPerm     string `json:"data_perm"` // all / cross_dept / dept / personal
	CompanyID    int64  `json:"company_id"`
	CrossDeptIDs string `json:"cross_dept_ids"` // 跨部门数据权限-部门ID列表（逗号分隔）
	jwt.RegisteredClaims
}

// JWTService 负责 token 的签发与解析。
type JWTService struct {
	secret      string
	expireHours int
}

// NewJWTService 基于配置构造 JWTService。
func NewJWTService(cfg *config.JWTConfig) *JWTService {
	return &JWTService{
		secret:      cfg.Secret,
		expireHours: cfg.ExpireHours,
	}
}

// GenerateToken 签发 HS256 token。
func (s *JWTService) GenerateToken(
	userID int64,
	username, roleCode string,
	deptID int64,
	dataPerm string,
	companyID int64,
	crossDeptIDs string,
) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:       userID,
		Username:     username,
		RoleCode:     roleCode,
		DeptID:       deptID,
		DataPerm:     dataPerm,
		CompanyID:    companyID,
		CrossDeptIDs: crossDeptIDs,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(s.expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "ai-camera",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

// ParseToken 解析并校验 token，返回 claims。
func (s *JWTService) ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(s.secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("无效的Token")
	}
	return claims, nil
}
