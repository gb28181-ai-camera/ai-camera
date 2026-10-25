package iot

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"os"
	"strings"

	"github.com/boticz/ai-camera-backend/internal/bootstrap"
	"github.com/boticz/ai-camera-backend/internal/model"
	"gorm.io/gorm/clause"
)

// TurnUserService 把产品×TURN 服务凭据同步到 coturn 的 SQL userdb。
//
// 为什么不用 coturn REST 凭据：产品×服务要求“每产品一套固定 TURN 用户名/密码”，
// 而 coturn 没有 HTTP/RPC 回调。coturn 原生支持 SQL userdb（turnusers_lt），
// 后端写入、coturn 查询，等价于动态账号且不需要维护 coturn 私有分支。
type TurnUserService struct {
	realm string
}

// NewTurnUserService 构造 TURN 用户同步服务。
func NewTurnUserService() *TurnUserService {
	realm := strings.TrimSpace(os.Getenv("TURN_REALM"))
	if realm == "" {
		realm = "ai-camera"
	}
	return &TurnUserService{realm: realm}
}

// Enabled 是否具备同步条件（当前始终启用；保留以便部署时显式关闭）。
func (s *TurnUserService) Enabled() bool { return s.realm != "" }

// Upsert 写入/更新 coturn 用户。
func (s *TurnUserService) Upsert(ctx context.Context, svc model.IotService, row model.IotProductService) error {
	if !s.Enabled() || svc.Protocol != model.IotProtocolTURN {
		return nil
	}
	key := md5Hex(row.Username + ":" + s.realm + ":" + row.Password)
	return bootstrap.DB().WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "realm"}, {Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"hmackey"}),
		}).
		Create(&model.TurnUserLT{Realm: s.realm, Name: row.Username, Hmackey: key}).Error
}

// Delete 删除 coturn 用户（产品解绑/删除/轮换旧账号时）。
func (s *TurnUserService) Delete(ctx context.Context, svc model.IotService, row model.IotProductService) {
	if !s.Enabled() || svc.Protocol != model.IotProtocolTURN {
		return
	}
	_ = bootstrap.DB().WithContext(ctx).
		Where("realm = ? AND name = ?", s.realm, row.Username).
		Delete(&model.TurnUserLT{}).Error
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
