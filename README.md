# go-cnb.cool — CNB OpenAPI Go SDK

[![CI](https://github.com/zy84338719/go-cnb.cool/actions/workflows/ci.yml/badge.svg)](https://github.com/zy84338719/go-cnb.cool/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/zy84338719/go-cnb.cool.svg)](https://pkg.go.dev/github.com/zy84338719/go-cnb.cool)
[![Go Version](https://img.shields.io/badge/go-1.22%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[CNB](https://cnb.cool)(云原生构建 / Cloud Native Build)OpenAPI 的 Go SDK。

**全量覆盖 [api.cnb.cool](https://api.cnb.cool) 公开接口 — 259 个操作、31 个服务分组、318 个数据模型**，由官方 [swagger.json](https://api.cnb.cool/swagger.json) 全自动生成，纯标准库实现，**零第三方依赖**。

## 特性

- 🛰️ **全量覆盖** — Issue / PR / Git 数据 / 仓库与组织 / 构建流水线 / 制品库 / 任务集 / 工作区 / AI / 知识库……全部 259 个接口
- 📦 **零依赖** — 只用 Go 标准库，`go get` 即用，无供应链负担
- 🔒 **类型安全** — 318 个数据模型、14 个枚举(`Visibility` / `AccessRole` / `RepoStatus` …)全部生成为具名类型
- 🧭 **地道 API** — go-github 风格:`ctx` 贯穿、`(结果, *Response, error)` 三返回值、`client.Issues.ListIssues(...)` 服务分组
- 🛡️ **PATCH 语义安全** — 请求体/查询参数字段全部指针 + `omitempty`，nil 不发送，绝不误清字段
- 📄 **结构化错误** — 解析 CNB 错误体 `{"errcode","errmsg","errparam"}` 为 `*ErrorResponse`，带 `IsNotFound()` 等判断
- 📑 **泛型分页** — `cnb.EachPage` 逐页遍历任意列表接口
- ♻️ **可持续更新** — spec 快照 + 生成器入库，一条命令重新生成，CI 自动校验生成代码与 spec 同步

## 安装

```bash
go get github.com/zy84338719/go-cnb.cool
```

包名是 `cnb`，导入时加个别名：

```go
import cnb "github.com/zy84338719/go-cnb.cool"
```

## 快速上手

```go
package main

import (
    "context"

    cnb "github.com/zy84338719/go-cnb.cool"
)

func main() {
    // 访问令牌: https://docs.cnb.cool/zh/develops/access-token.html
    client, err := cnb.NewClient("你的访问令牌")
    if err != nil {
        panic(err)
    }
    ctx := context.Background()

    // 当前用户
    me, _, _ := client.Users.GetUserInfo(ctx)
    println(me.Username)

    // 顶层组织列表 (查询参数)
    groups, _, _ := client.Organizations.ListTopGroups(ctx, &cnb.ListTopGroupsOptions{
        ListOptions: cnb.ListOptions{Page: 1, PageSize: 10},
        Role:        cnb.Ptr("Owner"), // 可选值见字段注释
    })

    // 查询仓库 Issue
    issues, _, _ := client.Issues.ListIssues(ctx, "org/repo", &cnb.ListIssuesOptions{
        ListOptions: cnb.ListOptions{PageSize: 20},
        State:       cnb.Ptr("open"),
        Labels:      cnb.Ptr("bug,feature"),
    })
    _ = issues

    // 创建 Issue (表单字段是指针: nil 不发送)
    issue, _, _ := client.Issues.CreateIssue(ctx, "org/repo", cnb.PostIssueForm{
        Title:  cnb.Ptr("SDK 上线啦"),
        Labels: []string{"announce"},
    })
    _ = issue

    // 合并 PR (squash)
    merged, _, _ := client.Pulls.MergePull(ctx, "org/repo", "12", cnb.MergePullRequest{
        MergeStyle: cnb.Ptr("squash"),
    })
    _ = merged
}
```

完整可运行示例见 [`examples/basic`](examples/basic/main.go)。

## 服务总览

| Client 字段 | 方法数 | 覆盖范围 |
|---|---:|---|
| `Git` | 35 | 分支/标签/提交/对比/原始内容/blob/commit 注解/归档下载/分支锁/LFS 预签名 |
| `Pulls` | 34 | 合并请求全生命周期: 增删改查、评论、标签、处理人、评审、合并、commit 状态 |
| `Issues` | 32 | Issue 及评论/标签/处理人/属性、动态、附件上传两段式接口 |
| `Members` | 20 | 组织/仓库/任务集成员管理、权限级别、外部协作者 |
| `Repositories` | 14 | 仓库创建/更新/转移/归档、fork、置顶、公开搜索外的仓库列表 |
| `Releases` | 13 | Release 与附件 (含预签名上传两段式接口) |
| `Organizations` | 12 | 组织创建/更新/转移/删除、子组织、logo 上传、组织设置 |
| `Build` | 11 | 云原生构建: 触发/停止/状态/日志/AI 审计/定时同步 |
| `GitSettings` | 11 | 分支保护、云原生构建设置、PR 设置、推送限制 |
| `Registries` | 10 | 制品库: 包/标签查询删除、描述更新、provenance |
| `Missions` | 8 | 任务集: 视图配置、视图列表、创建/删除 |
| `Users` | 6 | 用户信息、邮箱、GPG 密钥、自动补全 |
| `KnowledgeBase` | 6 | 知识库信息/查询/删除、embedding 模型 |
| `Workspace` | 5 | 云开发工作区: 启动/停止/删除/列表/详情 |
| `Rank` | 5 | 仓库榜单 (日/周/月/年) 与语言列表 |
| `Activities` / `Assets` / `Charge` / `Labels` | 4×4 | 用户动态与贡献者、附件资源、配额用量、仓库标签 |
| `Starring` / `Badge` / `NpcObservability` | 3×3 | 星标、徽章、NPC 可观测性 |
| `Followers` / `MissionResources` / `RepoCodeIssue` | 2×3 | 关注列表、任务资源、代码 Issue |
| `Events` / `Search` / `Security` / `ArtifactSecurity` / `RepoContributor` / `AI` | 1×6 | 仓库事件、公开仓库搜索、安全概览、制品扫描、贡献者趋势、AI 对话 |

## 分页

CNB 分页参数为 `page`(从 1 起)/ `page_size`(默认 10,上限 100)，响应为裸数组、不返回总数。所有列表类 Options 内嵌 `cnb.ListOptions`，用 `cnb.EachPage` 逐页遍历：

```go
opts := &cnb.ListPullsOptions{ListOptions: cnb.ListOptions{PageSize: 100}}
err := cnb.EachPage(100, func(page int) ([]*cnb.PullRequest, error) {
    opts.Page = page
    prs, _, err := client.Pulls.ListPulls(ctx, "org/repo", opts)
    return prs, err
}, func(prs []*cnb.PullRequest) error {
    for _, pr := range prs {
        println(pr.Title)
    }
    return nil
})
```

## 错误处理

非 2xx/3xx 响应返回 `*cnb.ErrorResponse`(对应 CNB 错误体 `{"errcode":..,"errmsg":..,"errparam":{..}}`):

```go
issue, _, err := client.Issues.GetIssue(ctx, "org/repo", 42)
if err != nil {
    var apiErr *cnb.ErrorResponse
    if cnb.AsErrorResponse(err, &apiErr) {
        switch {
        case apiErr.IsNotFound():     // 404
        case apiErr.IsUnauthorized(): // 401, 令牌无效
        case apiErr.IsForbidden():    // 403, 权限不足
        }
        log.Printf("errcode=%d errmsg=%s param=%v", apiErr.ErrCode, apiErr.ErrMsg, apiErr.ErrParam)
    }
}
```

## 文件与归档下载

无 JSON schema 的接口(归档、原始文件、图片、构建日志、Release/commit 附件、LFS 预签名)返回 `*Response`，响应体已缓冲——`resp.Body` 可按流读一次，`resp.BodyBytes()` 可随时多次取:

```go
// ref_with_path 支持: 分支名 / 标签名 / 提交哈希 / 分支名/文件路径 等
resp, err := client.Git.GetArchive(ctx, "org/repo", "main")
if err != nil { panic(err) }
data, _ := io.ReadAll(resp.Body) // tar/zip 归档内容
// 或者
data, _ := resp.BodyBytes()      // 缓冲副本, 可多次调用
```

> 302 预签名下载由 `http.Client` 自动跟随、直接拿到内容；需要 URL 本身时可用 `cnb.WithHTTPClient` 注入关闭重定向的 client。

## 注意事项

- **Issue/PR 路径参数 `number` 为 int**(spec 如此定义)，但返回模型里的 `Number` 字段是 string —— CNB API 自身如此，SDK 原样保留。
- 请求体(Form)与查询参数(Options)字段均为指针 + `omitempty`: `nil` 不发送，空串/0/false 会发送。
- 响应模型中的枚举字段是具名类型(如 `OrganizationAccess.AccessRole` 为 `cnb.AccessRole`)，常量形如 `cnb.AccessRoleOwner`、`cnb.VisibilityPublic`;查询参数里的枚举值(如 `Role`)是 `*string`，直接传字符串。

## 重新生成

SDK 由 [`internal/gen/generate.py`](internal/gen/generate.py) 从官方 swagger.json 全量生成，CI 会自动校验生成代码与 spec 同步:

```bash
# 更新 API 覆盖: 重新下载 spec 后重新生成
python3 internal/gen/generate.py
go build ./... && go test ./...
```

生成文件(`*_gen.go`, 带 DO NOT EDIT 头)与手写文件(`cnb.go` / `errors.go` / `params.go` / `pagination.go` / `util.go` / 测试 / 示例)分工见 [AGENTS.md](AGENTS.md)。

## 开发

```bash
go build ./... && go vet ./... && go test ./...   # 本地跑 CI 同款检查
go run ./examples/basic                            # 需要 CNB_TOKEN 环境变量
```

## License

[MIT](LICENSE)
