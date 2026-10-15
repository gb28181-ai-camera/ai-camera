package router

import (
	"github.com/gin-gonic/gin"
)

// registerSwaggerRoutes 注册 Swagger 文档路由。
//
// docs/web 与 docs/app 两个包由 `bash scripts/gen-swagger.sh` 生成，
// 通过空白 import 触发其 init() 把文档注册到 swag。
// 首次生成前这两个包不存在，因此这里以动态方式提供 JSON 静态文件服务，
// 保证未生成文档时服务仍可正常启动。
func registerSwaggerRoutes(r *gin.Engine) {
	// Swagger UI 页面
	r.StaticFile("/swagger-ui", "static/swagger-ui/index.html")

	// API 文档 JSON（由 gen-swagger.sh 输出到 docs/web 与 docs/app）
	//
	// 注意文件名：swag init 使用 --instanceName 时会把产物命名为
	// <instanceName>_swagger.json（即 web_swagger.json / app_swagger.json），
	// 而非默认的 swagger.json。文件名不一致会导致此处 404。
	r.StaticFile("/web/api-docs.json", "docs/web/web_swagger.json")
	r.StaticFile("/app/api-docs.json", "docs/app/app_swagger.json")
}
