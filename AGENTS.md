# AGENTS.md — go-cnb

CNB (cnb.cool) OpenAPI 的 Go SDK。纯标准库，零依赖。Go 1.22+。

## 结构

- `*_gen.go` — **生成文件，勿手改**。由 `python3 internal/gen/generate.py` 从 `internal/gen/swagger.json`（官方 https://api.cnb.cool/swagger.json 快照）生成：
  - `client_gen.go`: Client 结构体 + 全部 Service + initServices
  - `<service>_gen.go`: 259 个 API 方法（31 个服务）
  - `options_gen.go`: 查询参数 Options / 内联请求体
  - `models_*_gen.go`: 318 个数据模型；`enums_gen.go`: 14 个枚举
- 手写文件：`cnb.go`（NewClient/NewRequest/Do）、`errors.go`（ErrorResponse）、`params.go`（addQuery）、`pagination.go`（ListOptions/EachPage）、`util.go`（Ptr/escapePath）、`cnb_test.go`

## 约定

- 模块路径 `github.com/zy84338719/go-cnb.cool`（GitHub: https://github.com/zy84338719/go-cnb.cool ），包名 `cnb`，导入需别名
- 请求体/查询参数字段：指针 + omitempty；响应模型：值字段（请求可达的 dto 例外，为指针风格）
- Issue/PR 的 `number` 是 **string**（API 如此定义）
- 无 schema 的下载接口返回 `*Response`，body 已缓冲可重复读
- 更新 API 覆盖：重新下载 swagger.json → 放入 internal/gen/ → 跑生成器 → `go build ./... && go test ./...`

## 命令

```bash
go build ./... && go vet ./... && go test ./...
python3 internal/gen/generate.py   # 重新生成
go run ./examples/basic            # 需要 CNB_TOKEN 环境变量
```

## CI

`.github/workflows/ci.yml`：push/PR 触发
- **test**：Go 1.22.x / stable 矩阵跑 gofmt 检查 + build + vet + test
- **generator-check**：重跑生成器并 `git diff --exit-code` 校验 `*_gen.go` 与 spec 同步

改生成器或 spec 后必须重新生成并一并提交，否则 generator-check 会红。
