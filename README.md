# go-cnb

[CNB](https://cnb.cool) (云原生构建 / Cloud Native Build) OpenAPI 的 Go SDK。
全量覆盖 `https://api.cnb.cool` 公开接口 — **259 个操作、31 个服务分组、318 个数据模型**，纯标准库实现，零第三方依赖。

- API 文档: <https://api.cnb.cool> (spec: <https://api.cnb.cool/swagger.json>)
- OpenAPI 使用说明: <https://docs.cnb.cool/zh/develops/openapi.html>
- 访问令牌: <https://docs.cnb.cool/zh/develops/access-token.html>

## 安装

```bash
go get github.com/zy84338719/go-cnb.cool
```

包名是 `cnb`，导入时加个别名即可：

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
    client, err := cnb.NewClient("你的访问令牌") // 创建: https://docs.cnb.cool/zh/develops/access-token.html
    if err != nil {
        panic(err)
    }
    ctx := context.Background()

    // 我的信息
    me, _, err := client.Users.GetUserInfo(ctx)

    // 组织列表 (查询参数)
    groups, _, _ := client.Organizations.ListTopGroups(ctx, &cnb.ListTopGroupsOptions{
        ListOptions: cnb.ListOptions{Page: 1, PageSize: 10},
    })

    // 仓库 Issue
    issues, _, _ := client.Issues.ListIssues(ctx, "cnb/awesome-cnb", &cnb.ListIssuesOptions{
        State:   cnb.Ptr("open"),
        Keyword: cnb.Ptr("docker"),
    })

    // 创建 Issue (请求体字段是指针, nil 即不发送, PATCH 语义安全)
    issue, _, _ := client.Issues.CreateIssue(ctx, "org/repo", cnb.PostIssueForm{
        Title:   cnb.Ptr("标题"),
        Body:    cnb.Ptr("内容"),
        Labels:  []string{"bug"},
    })

    // 合并 PR
    merged, _, _ := client.Pulls.MergePull(ctx, "org/repo", "12", cnb.MergePullRequest{
        MergeStyle: cnb.Ptr("squash"),
    })
}
```

完整示例见 [`examples/basic`](examples/basic)。

## 服务分组

| Client 字段 | 覆盖范围 |
|---|---|
| `Issues` / `Pulls` | Issue / 合并请求: 增删改查、评论、标签、处理人、评审、合并、动态 |
| `Git` | 分支/标签/提交/对比/Blame 原始内容、blob 创建、commit 注解、归档下载、LFS 预签名 |
| `Repositories` / `Organizations` | 仓库/组织: 创建、设置、转移、归档、fork、置顶 |
| `Members` / `Collaborators`(并入 Members) | 成员管理、权限级别、外部协作者 |
| `Releases` | Release 与附件 (含预签名上传两段式接口) |
| `GitSettings` | 分支保护、云原生构建、PR、推送限制设置 |
| `Build` | 流水线构建: 触发、状态、日志、AI 审计、定时同步 |
| `Registries` | 制品库: 包/标签查询删除、provenance |
| `Missions` / `MissionResources` | 任务集与任务资源 |
| `KnowledgeBase` / `AI` | 知识库、AI 对话 |
| `Users` / `Followers` / `Starring` | 用户信息、邮箱、GPG、关注、星标 |
| `Workspace` | 云开发工作区 |
| `Activities` / `Events` / `Rank` / `Search` | 动态、仓库事件、榜单、公开仓库搜索 |
| `Assets` / `Badge` / `Labels` / `Charge` / `Security` / `ArtifactSecurity` / `NpcObservability` / `RepoCodeIssue` / `RepoContributor` | 附件、徽章、标签、配额、安全概览等 |

## 分页

CNB 分页参数为 `page` (从 1 起) / `page_size` (默认 10, 上限 100)，响应为裸数组、不返回总数。所有列表类 Options 都内嵌 `cnb.ListOptions`，可用 `cnb.EachPage` 逐页遍历：

```go
opts := &cnb.ListPullsOptions{ListOptions: cnb.ListOptions{PageSize: 100}}
err := cnb.EachPage(100, func(page int) ([]*cnb.PullRequest, error) {
    opts.Page = page
    prs, _, err := client.Pulls.ListPulls(ctx, "org/repo", opts)
    return prs, err
}, func(prs []*cnb.PullRequest) error {
    // 处理每一页
    return nil
})
```

## 错误处理

非 2xx/3xx 响应返回 `*cnb.ErrorResponse`（CNB 错误体 `{"errcode":..,"errmsg":..,"errparam":{..}}`）：

```go
issue, _, err := client.Issues.GetIssue(ctx, "org/repo", 42)
if err != nil {
    var apiErr *cnb.ErrorResponse
    if cnb.AsErrorResponse(err, &apiErr) && apiErr.IsNotFound() {
        // 不存在
    }
}
```

## 文件/归档下载

无 JSON schema 的接口（归档、原始文件、图片、构建日志、LFS/Release 预签名等）返回 `*Response`，响应体已缓冲，直接读 `resp.Body`：

```go
// ref_with_path 支持: 分支名 / 标签名 / 提交哈希 / 分支名/文件路径 等
resp, err := client.Git.GetArchive(ctx, "org/repo", "main")
if err != nil { panic(err) }
data, _ := io.ReadAll(resp.Body) // tar/zip 归档内容
```

## 注意事项

- **Issue / PR 路径参数 `number` 为 int**（spec 如此定义），但返回模型里的 `Number` 字段是 string —— CNB API 自身如此，SDK 原样保留。
- 请求体 (Form) 与查询参数 (Options) 字段均为指针 + `omitempty`：`nil` 不发送，空串/0/false 会发送。
- 枚举（`Visibility`、`AccessRole`、`RepoStatus` 等 14 个）生成为具名类型和常量。
- 302 预签名下载（Release 附件、commit 附件、LFS）由 `http.Client` 自动跟随后直接得到内容；需要 URL 本身时请自行用 `WithHTTPClient` 关闭重定向。

## 重新生成

SDK 由 `internal/gen/generate.py` 从官方 swagger.json 全量生成：

```bash
# 更新 spec 后重新生成 (生成文件带 DO NOT EDIT 头)
python3 internal/gen/generate.py
go build ./... && go test ./...
```

手写文件：`cnb.go`（Client 核心逻辑）、`errors.go`、`params.go`、`pagination.go`、`util.go`、测试与示例。

## License

MIT
