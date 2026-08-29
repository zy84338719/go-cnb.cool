# go-cnb.cool — CNB OpenAPI Go SDK

[![CI](https://github.com/zy84338719/go-cnb.cool/actions/workflows/ci.yml/badge.svg)](https://github.com/zy84338719/go-cnb.cool/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/zy84338719/go-cnb.cool.svg)](https://pkg.go.dev/github.com/zy84338719/go-cnb.cool)
[![Release](https://img.shields.io/github/v/release/zy84338719/go-cnb.cool?color=blue)](https://github.com/zy84338719/go-cnb.cool/releases)
[![Go Version](https://img.shields.io/badge/go-1.22%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

类型安全、**全量覆盖**的 [CNB](https://cnb.cool)(云原生构建 / Cloud Native Build)OpenAPI Go 客户端。

由官方 [swagger.json](https://api.cnb.cool/swagger.json) 自动生成 —— **259 个接口 · 31 个服务分组 · 318 个数据模型 · 14 个枚举**，纯标准库实现，零第三方依赖。

| | |
|---|---|
| 🛰️ 全量覆盖 | Issue / PR / Git / 构建 / 制品 / 任务集 / 工作区 / AI……api.cnb.cool 公开接口一个不落 |
| 📦 零依赖 | 只用 Go 标准库，无供应链负担 |
| 🔒 类型安全 | 318 个模型 + 14 个枚举全部生成为具名类型和常量 |
| 🧭 地道 API | go-github 风格:`ctx` 贯穿、`(结果, *Response, error)`、服务分组 |
| 🛡️ PATCH 安全 | 请求/查询字段指针 + `omitempty`，nil 不发送，绝不误清字段 |
| 📄 结构化错误 | `{"errcode","errmsg","errparam"}` → `*ErrorResponse`，带 `IsNotFound()` 等判断 |
| 📑 泛型分页 | `cnb.EachPage` 逐页遍历任意列表接口 |
| ♻️ 持续同步 | spec 快照 + 生成器入库，CI 自动校验生成代码与 spec 一致 |

## 目录

- [安装](#安装)
- [快速开始](#快速开始)
- [认证](#认证)
- [常用场景](#常用场景)
- [分页](#分页)
- [错误处理](#错误处理)
- [文件与归档下载](#文件与归档下载)
- [枚举与模型](#枚举与模型)
- [高级配置](#高级配置)
- [API 覆盖总览](#api-覆盖总览)
- [注意事项](#注意事项)
- [开发与再生成](#开发与再生成)

## 安装

```bash
go get github.com/zy84338719/go-cnb.cool
```

包名是 `cnb`，导入时加个别名：

```go
import cnb "github.com/zy84338719/go-cnb.cool"
```

## 快速开始

```go
package main

import (
    "context"
    "fmt"

    cnb "github.com/zy84338719/go-cnb.cool"
)

func main() {
    ctx := context.Background()

    client, err := cnb.NewClient("你的访问令牌")
    if err != nil {
        panic(err)
    }

    // 我是谁
    me, _, err := client.Users.GetUserInfo(ctx)
    if err != nil {
        panic(err)
    }
    fmt.Println("当前用户:", me.Username)

    // 我有权限的组织
    groups, _, err := client.Organizations.ListTopGroups(ctx, &cnb.ListTopGroupsOptions{
        ListOptions: cnb.ListOptions{Page: 1, PageSize: 10},
    })
    if err != nil {
        panic(err)
    }
    for _, g := range groups {
        fmt.Printf("组织: %s（成员 %d · 仓库 %d）\n", g.Name, g.AllMemberCount, g.AllSubRepoCount)
    }

    // 某仓库的开放 Issue
    issues, _, err := client.Issues.ListIssues(ctx, "org/repo", &cnb.ListIssuesOptions{
        ListOptions: cnb.ListOptions{PageSize: 20},
        State:       cnb.Ptr("open"),
        Labels:      cnb.Ptr("bug,feature"),
    })
    if err != nil {
        panic(err) // 例: 仓库不存在 / 令牌无权限
    }
    for _, is := range issues {
        fmt.Printf("#%s %s\n", is.Number, is.Title)
    }
}
```

可运行示例:
- [`examples/basic`](examples/basic/main.go)——用户信息 / 组织 / Issue 遍历
- [`examples/issue-triage`](examples/issue-triage/main.go)——自动给无标签 Issue 打标 + 评论(写操作 + 分页 + 错误处理实战)

## 认证

SDK 使用 CNB **访问令牌**进行 Bearer 认证，令牌创建与权限说明见官方文档:
<https://docs.cnb.cool/zh/develops/access-token.html>

```go
client, err := cnb.NewClient("1Z00000000000000000000000vA")
```

每个生成方法的 godoc 注释里都标注了该接口需要的令牌权限(如 `repo-issue:r`、`account-engage:r`)，创建令牌时按需勾选即可。

## 常用场景

### Issue:创建 / 更新 / 评论

```go
// 创建 (labels 直接赋值, 其余可选字段用 cnb.Ptr)
issue, _, err := client.Issues.CreateIssue(ctx, "org/repo", cnb.PostIssueForm{
    Title:  cnb.Ptr("支持导出报表"),
    Body:   cnb.Ptr("希望能导出 CSV"),
    Labels: []string{"feature"},
})

// 局部更新: 只发送要改的字段, nil 字段不会出现在请求体里
_, _, err = client.Issues.UpdateIssue(ctx, "org/repo", 42, cnb.PatchIssueForm{
    State:     cnb.Ptr("closed"),
    StateReason: cnb.Ptr("completed"),
})

// 评论
_, _, err = client.Issues.PostIssueComment(ctx, "org/repo", 42, cnb.PostIssueCommentForm{
    Body: cnb.Ptr("已修复, 请验证"),
})
```

### Pull Request:创建 / 合并 / 评审

```go
// 创建 PR (跨仓库 fork 场景用 HeadRepo 指定源仓库)
pr, _, err := client.Pulls.PostPull(ctx, "org/repo", cnb.PullCreationForm{
    Title: cnb.Ptr("feat: 报表导出"),
    Head:  cnb.Ptr("feat/export"),
    Base:  cnb.Ptr("main"),
})

// 合并方式: merge / squash / rebase
merged, _, err := client.Pulls.MergePull(ctx, "org/repo", "12", cnb.MergePullRequest{
    MergeStyle: cnb.Ptr("squash"),
})
```

### Git:分支 / 提交 / 文件内容 / 对比

```go
// 分支列表
branches, _, _ := client.Git.ListBranches(ctx, "org/repo", &cnb.ListBranchesOptions{
    ListOptions: cnb.ListOptions{PageSize: 100},
})

// 提交历史 (可按分支/SHA/时间/作者过滤)
commits, _, _ := client.Git.ListCommits(ctx, "org/repo", &cnb.ListCommitsOptions{
    Sha:   cnb.Ptr("main"),
    Since: cnb.Ptr("2026-01-01T00:00:00Z"),
})

// 读文件 (指定 ref), 内容为 base64, 用 DecodedContent 解码
content, _, _ := client.Git.GetContent(ctx, "org/repo", "README.md", &cnb.GetContentOptions{
    Ref: cnb.Ptr("main"),
})
src, err := content.DecodedContent() // []byte 文件内容; 目录用 content.Entries

// 两点对比 (格式 base...head)
diff, _, _ := client.Git.GetCompareCommits(ctx, "org/repo", "main...dev")
```

### 构建流水线:触发 / 状态 / 日志

```go
// 触发构建 (Event 须为 api_trigger 或以其开头)
build, _, err := client.Build.StartBuild(ctx, "org/repo", cnb.StartBuildReq{
    Branch: cnb.Ptr("main"),
    Event:  cnb.Ptr("api_trigger"),
    Env:    map[string]string{"FOO": "bar"},
})

// 查询状态 / 拉取日志
status, _, _ := client.Build.GetBuildStatus(ctx, "org/repo", build.Sn)
logs, _, _ := client.Build.GetBuildLogs(ctx, "org/repo", &cnb.GetBuildLogsOptions{
    Event: cnb.Ptr("push"),
})
```

### 制品库:包与标签

```go
// 列出 docker 制品
pkgs, _, _ := client.Registries.ListPackages(ctx, "org", &cnb.ListPackagesOptions{
    Type: cnb.Ptr("docker"), // all/helm/maven/npm/pypi/...
})

// 包的标签
tags, _, _ := client.Registries.ListPackageTags(ctx, "org", "docker", "my-app", nil)
```

### 组织与成员

```go
// 创建组织
_, err := client.Organizations.CreateOrganization(ctx, cnb.CreateGroupReq{
    Path:        cnb.Ptr("my-org"),
    Description: cnb.Ptr("我的组织"),
})

// 添加成员并授权
_, err = client.Members.AddMembersOfGroup(ctx, "my-org", "someone", cnb.UpdateMembersRequest{
    AccessLevel: cnb.Ptr("Developer"), // Guest/Reporter/Developer/Master/Owner
})
```

### AI:流式对话

部分 CNB AI 模型仅支持流式返回，请用流式方法 + `cnb.ScanSSE` 消费 SSE 事件:

```go
resp, err := client.AI.AiChatCompletionsStream(ctx, "org/repo", cnb.AiChatCompletionsReq{
    Model:    cnb.Ptr("模型名"),
    Stream:   cnb.Ptr(true),
    Messages: []cnb.Message{{Role: cnb.Ptr("user"), Content: cnb.Ptr("你好")}},
})
if err != nil {
    panic(err)
}
err = cnb.ScanSSE(resp.Body, func(ev cnb.SSEEvent) error {
    fmt.Println(ev.Data) // JSON 增量; "[DONE]" 表示结束
    return nil           // 返回 error 提前终止
})
```

## 分页

CNB 分页参数为 `page`(从 1 起)/ `page_size`(默认 10,上限 100)，响应为裸数组、**不返回总数**。所有列表类 Options 内嵌 `cnb.ListOptions`，用 `cnb.EachPage` 逐页遍历，直到空页或不足一页:

```go
opts := &cnb.ListPullsOptions{ListOptions: cnb.ListOptions{PageSize: 100}}
err := cnb.EachPage(100, func(page int) ([]*cnb.PullRequest, error) {
    opts.Page = page
    prs, _, err := client.Pulls.ListPulls(ctx, "org/repo", opts)
    return prs, err
}, func(prs []*cnb.PullRequest) error {
    for _, pr := range prs {
        fmt.Println(pr.Title)
    }
    return nil // 返回 error 可提前终止
})
```

## 错误处理

非 2xx/3xx 响应统一返回 `*cnb.ErrorResponse`(对应 CNB 错误体)，用 `errors.As` / `cnb.AsErrorResponse` 断言:

```go
issue, _, err := client.Issues.GetIssue(ctx, "org/repo", 42)
if err != nil {
    var apiErr *cnb.ErrorResponse
    if cnb.AsErrorResponse(err, &apiErr) {
        log.Printf("errcode=%d errmsg=%s param=%v",
            apiErr.ErrCode, apiErr.ErrMsg, apiErr.ErrParam)
        switch {
        case apiErr.IsUnauthorized(): // 401 令牌无效/过期
        case apiErr.IsForbidden():    // 403 令牌权限不足
        case apiErr.IsNotFound():     // 404 资源不存在
        }
    }
    return err
}
```

非 JSON 错误体(网关错误等)时原文保留在 `apiErr.RawBody`，错误信息含 HTTP 状态码。

## 文件与归档下载

无 JSON schema 的接口(归档、原始文件、图片、构建日志、Release/commit 附件、LFS 预签名)返回 `*Response`，响应体已缓冲——`resp.Body` 可按流读一次，`resp.BodyBytes()` 可随时多次取:

```go
// ref_with_path 支持: 分支名 / 标签名 / 提交哈希 / 分支名/文件路径
resp, err := client.Git.GetArchive(ctx, "org/repo", "main")
if err != nil {
    panic(err)
}
data, _ := io.ReadAll(resp.Body) // tar/zip 归档内容
// 或 data, _ := resp.BodyBytes()

// 原始文件内容
raw, _, err := client.Git.GetRaw(ctx, "org/repo", "main/Dockerfile", nil)
```

> Release/commit 附件与 LFS 是 302 预签名地址，`http.Client` 默认自动跟随、直接拿到内容；需要 URL 本身时，用 `cnb.WithHTTPClient` 注入关闭重定向的 client。

## 枚举与模型

响应模型中的枚举字段是具名类型，常量形如:

```go
// 响应模型的枚举字段是具名类型, 与常量直接比较
if group.AccessRole == cnb.AccessRoleOwner { /* ... */ }
if repo.Visibility == cnb.VisibilityPrivate { /* ... */ }

// int 枚举同样是具名常量
if repo.Status == cnb.RepoStatusArchived { /* ... */ }
```

全部 14 个枚举:`AccessRole` `Visibility` `RepoStatus` `UserType` `SlugType` `PackageType` `AssetRecordType` `InteractionType` `MissionViewSortOrder` `MissionViewType` `OperatorType` `Repo`(功能开关) `ChannelTypeTarget` `SettingValue`。

模型字段类型忠实于 CNB API:嵌套对象为具名结构体，动态字段为 `map[string]any`，时间字段为 `string`。

## 高级配置

```go
client, err := cnb.NewClient(token,
    // 自定义 *http.Client: 超时/代理/重定向策略/重试中间件
    cnb.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),

    // 私有化部署或代理地址 (支持带路径前缀, 如 https://gw.internal/api/)
    cnb.WithBaseURL("https://api.example.com/"),
)
if err != nil {
    panic(err)
}
client.UserAgent = "my-bot/1.0"                   // 默认 "go-cnb"
client.Accept = "application/vnd.cnb.api+json"     // 默认 "application/json"
```

**自动重试**(可选,`cnb.WithRetry`):

```go
client, _ := cnb.NewClient(token, cnb.WithRetry(3))
```

- `429/502/503/504` 所有方法自动重试,`429` 优先遵守 `Retry-After`
- 网络层错误(连接失败/超时)仅幂等的 `GET/HEAD` 重试
- 指数退避(200ms 起,上限 3s)+ 抖动;其余(含 4xx 业务错误)不重试

## API 覆盖总览

| Client 字段 | 方法数 | 覆盖范围 |
|---|---:|---|
| `Git` | 35 | 分支/标签/提交/对比/原始内容/blob/commit 注解/归档下载/分支锁/LFS 预签名 |
| `Pulls` | 34 | 合并请求全生命周期: 增删改查、评论、标签、处理人、评审、合并、commit 状态 |
| `Issues` | 32 | Issue 及评论/标签/处理人/属性、动态、附件上传两段式接口 |
| `Members` | 20 | 组织/仓库/任务集成员管理、权限级别、外部协作者 |
| `Repositories` | 14 | 仓库创建/更新/转移/归档、fork、置顶 |
| `Releases` | 13 | Release 与附件(含预签名上传两段式接口) |
| `Organizations` | 12 | 组织创建/更新/转移/删除、子组织、logo 上传、组织设置 |
| `Build` | 11 | 云原生构建: 触发/停止/状态/日志/AI 审计/定时同步 |
| `GitSettings` | 11 | 分支保护、云原生构建设置、PR 设置、推送限制 |
| `Registries` | 10 | 制品库: 包/标签查询删除、描述更新、provenance |
| `Missions` | 8 | 任务集: 视图配置、视图列表、创建/删除 |
| `Users` | 6 | 用户信息、邮箱、GPG 密钥、自动补全 |
| `KnowledgeBase` | 6 | 知识库信息/查询/删除、embedding 模型 |
| `Workspace` | 5 | 云开发工作区: 启动/停止/删除/列表/详情 |
| `Rank` | 5 | 仓库榜单(日/周/月/年)与语言列表 |
| `Activities` / `Assets` / `Charge` / `Labels` | 4×4 | 用户动态与贡献者、附件资源、配额用量、仓库标签 |
| `Starring` / `Badge` / `NpcObservability` | 3×3 | 星标、徽章、NPC 可观测性 |
| `Followers` / `MissionResources` / `RepoCodeIssue` | 2×3 | 关注列表、任务资源、代码 Issue |
| `Events` / `Search` / `Security` / `ArtifactSecurity` / `RepoContributor` / `AI` | 1×6 | 仓库事件、公开仓库搜索、安全概览、制品扫描、贡献者趋势、AI 对话 |

## 注意事项

- **Issue/PR 路径参数 `number` 为 int**(spec 如此定义)，但返回模型里的 `Number` 字段是 string —— CNB API 自身如此，SDK 原样保留。
- 请求体(Form)与查询参数(Options)字段均为指针 + `omitempty`:`nil` 不发送，空串/0/false 会发送。用 `cnb.Ptr(...)` 构造。
- 请求/查询侧的枚举值(如 `ListTopGroupsOptions.Role`、`CreateGroupReq.Visibility`)是 `*string`，直接传字符串(`cnb.Ptr("Owner")`);响应模型里的枚举字段才是具名枚举类型，可与 `cnb.AccessRoleOwner` 等常量直接比较。
- SDK 并发安全:一个 `Client` 可在多个 goroutine 复用。

## 开发与再生成

```bash
git clone https://github.com/zy84338719/go-cnb.cool
cd go-cnb.cool

golangci-lint run ./...    # lint (CI 同款)
go test -race ./...        # 38+ 测试: 路由表全量回归 + 正例解码 + 边界行为
go run ./examples/basic    # 需要环境变量 CNB_TOKEN

# 真实 API 集成测试 (不设 CNB_TOKEN 自动跳过)
CNB_TOKEN=你的访问令牌 go test -run Integration -v .
```

SDK 由 [`internal/gen/generate.py`](internal/gen/generate.py) 从官方 swagger.json 全量生成(spec 快照入库)。CNB 更新接口后:

```bash
# 1. 下载最新 spec
python3 -c "import urllib.request; urllib.request.urlretrieve('https://api.cnb.cool/swagger.json', 'internal/gen/swagger.json')"
# 2. 重新生成 + 验证
python3 internal/gen/generate.py
go build ./... && go test ./...
# 3. 提交 (CI 的 generator-check 会校验生成代码与 spec 同步)
```

CI 四道门:`Lint` · `Test (Go 1.22.x / stable)` · `Generated code in sync`。
路由表测试会把 259 个接口逐一真实调用，断言方法存在、HTTP 动词与路径模板与 spec 完全一致。

## License

[MIT](LICENSE) © [zy84338719](https://github.com/zy84338719)
