package model

import (
	"context"
	"reflect"
	"time"

	"gorm.io/gorm"
)

// BaseModel 被所有业务模型嵌入，提供主键、软删除与审计字段。
type BaseModel struct {
	ID        int64          `gorm:"primarykey" json:"id"`
	DeletedAt gorm.DeletedAt `gorm:"index;comment:软删除时间" json:"deleted_at,omitempty"`

	CreatedBy string    `gorm:"column:created_by;size:50;comment:创建人" json:"created_by,omitempty"`
	CreatedDT time.Time `gorm:"column:created_dt;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_dt"`
	UpdatedBy string    `gorm:"column:updated_by;size:50;comment:更新人" json:"updated_by,omitempty"`
	UpdatedDT time.Time `gorm:"column:updated_dt;default:CURRENT_TIMESTAMP;autoUpdateTime;comment:更新时间" json:"updated_dt"`
}

// ────────────────────────────────────────────────────────────────
// 请求上下文：由 Auth 中间件写入，供 GORM 钩子自动填充审计字段
// ────────────────────────────────────────────────────────────────

type ctxKey string

const (
	ctxKeyUsername ctxKey = "current_username"
	ctxKeyDeptID   ctxKey = "current_dept_id"
	ctxKeyUserID   ctxKey = "current_user_id"
)

// WithUsername 将当前登录名写入 context。
func WithUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, ctxKeyUsername, username)
}

// UsernameFromCtx 读取当前登录名，缺省为 "system"。
func UsernameFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyUsername).(string); ok && v != "" {
		return v
	}
	return "system"
}

// WithDeptID 将当前部门 ID 写入 context。
func WithDeptID(ctx context.Context, deptID int64) context.Context {
	return context.WithValue(ctx, ctxKeyDeptID, deptID)
}

// DeptIDFromCtx 读取当前部门 ID。
func DeptIDFromCtx(ctx context.Context) *int64 {
	if v, ok := ctx.Value(ctxKeyDeptID).(int64); ok && v > 0 {
		return &v
	}
	return nil
}

// WithUserID 将当前用户 ID 写入 context。
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, userID)
}

// UserIDFromCtx 读取当前用户 ID。
func UserIDFromCtx(ctx context.Context) *int64 {
	if v, ok := ctx.Value(ctxKeyUserID).(int64); ok && v > 0 {
		return &v
	}
	return nil
}

// ────────────────────────────────────────────────────────────────
// GORM 钩子：自动填充 created_by / updated_by / dept_id
// ────────────────────────────────────────────────────────────────

// BeforeCreate 自动填充创建人、更新人与部门（若模型含 dept_id 列且为空）。
func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if tx.Statement == nil || tx.Statement.Context == nil {
		return nil
	}
	ctx := tx.Statement.Context

	if b.CreatedBy == "" {
		b.CreatedBy = UsernameFromCtx(ctx)
	}
	if b.UpdatedBy == "" {
		b.UpdatedBy = UsernameFromCtx(ctx)
	}

	// 反射回填 dept_id：仅当目标结构体存在该字段且当前为 nil 时。
	// 部门归属是数据权限的基础，缺失会导致 dept 级过滤失效。
	fillDeptID(tx, DeptIDFromCtx(ctx))
	return nil
}

// BeforeUpdate 自动填充更新人。
func (b *BaseModel) BeforeUpdate(tx *gorm.DB) error {
	if tx.Statement == nil || tx.Statement.Context == nil {
		return nil
	}
	b.UpdatedBy = UsernameFromCtx(tx.Statement.Context)
	return nil
}

// fillDeptID 通过反射为含 DeptID 字段的模型回填部门。
func fillDeptID(tx *gorm.DB, deptID *int64) {
	if deptID == nil || tx.Statement == nil || tx.Statement.Dest == nil {
		return
	}

	v := reflect.ValueOf(tx.Statement.Dest)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	field := v.FieldByName("DeptID")
	if !field.IsValid() || !field.CanSet() {
		return
	}

	// 支持 *int64 与 int64 两种声明方式
	switch field.Kind() {
	case reflect.Ptr:
		if field.IsNil() {
			id := *deptID
			field.Set(reflect.ValueOf(&id))
		}
	case reflect.Int64:
		if field.Int() == 0 {
			field.SetInt(*deptID)
		}
	}
}
