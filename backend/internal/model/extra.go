package model

// 模块自注册模型。
//
// 背景：AutoMigrate 的模型清单集中在 bootstrap/migrate.go。多人/多 agent
// 并行开发时，每个模块都往同一处追加一行会产生频繁冲突。这里提供注册表：
// 模块在自己的 model 文件里写
//
//	func init() { RegisterModel(&Xxx{}) }
//
// bootstrap 在迁移时把 ExtraModels() 追加到主清单，注册顺序不影响建表
// （GORM AutoMigrate 会按依赖处理）。
//
// 约定：
//   - 只在 model 包内注册；不要在运行期反复注册，避免重复迁移。
//   - 新增表仍须同步 docs/开发文档/数据库设计/sql/ddl/ 下的 DDL。
var extraModels []interface{}

// RegisterModel 注册需要 AutoMigrate 的模型。
func RegisterModel(models ...interface{}) {
	extraModels = append(extraModels, models...)
}

// ExtraModels 返回模块自注册的模型清单。
func ExtraModels() []interface{} {
	return extraModels
}
