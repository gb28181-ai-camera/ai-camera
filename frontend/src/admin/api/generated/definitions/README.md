# src/admin/api/generated/definitions

本目录下的 `*.ts` **由 `scripts/api-gen.sh` 自动生成，请勿手工编辑**。

生成链路：

```
后端 Swagger 注解（internal/api/handler/web/*.go）
  → cd backend && bash scripts/gen-swagger.sh        # swag init → docs/web/web_swagger.json
  → cd frontend && bash scripts/api-gen.sh           # swagger2-doc-parser 或本地生成器
  → src/admin/api/generated/**                        # 自动生成
  → 前端页面引用 Controller 静态方法 / definitions 类型
```

- 生成器与参数说明见 [scripts/README.md](../../../../../scripts/README.md)
- 每个文件对应一个 swagger definition：`export type X = Definition & { ... }`
- 字段名与可选性照抄 swagger（`properties` + `required`），`enum` 展开为字面量联合类型
- 需要保留的手写类型（如 `OperationLogQuery` / `DashboardData`）登记在
  `scripts/api-gen-preserve.mjs`，不要直接改本目录的文件
