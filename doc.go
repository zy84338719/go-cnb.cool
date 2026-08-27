// Package cnb 是 CNB (cnb.cool, 云原生构建/代码托管平台) OpenAPI 的 Go SDK.
//
// 全量覆盖 https://api.cnb.cool 公开接口 (259 个操作, 31 个服务分组, 318 个数据模型),
// 由官方 swagger.json 自动生成, 纯标准库实现, 无第三方依赖.
//
// # 快速上手
//
//	client, err := cnb.NewClient("your-access-token")
//	groups, resp, err := client.Organizations.ListTopGroups(ctx, nil)
//
// 令牌创建: https://docs.cnb.cool/zh/develops/access-token.html
// API 文档: https://api.cnb.cool
//
// # 服务分组
//
// Client 上挂载 31 个服务, 与 CNB API 文档的分类一一对应:
//
//	Issues            Issue: 增删改查/评论/标签/处理人/属性/动态/附件上传
//	Pulls             合并请求: 全生命周期, 含评审与合并
//	Git               分支/标签/提交/对比/原始内容/归档下载/LFS
//	Members           组织/仓库/任务集成员与外部协作者
//	Repositories      仓库: 创建/更新/转移/归档/fork/置顶
//	Organizations     组织: 创建/更新/转移/子组织/设置
//	Releases          Release 与附件
//	Build             云原生构建流水线: 触发/状态/日志/AI 审计
//	GitSettings       分支保护/云原生构建/PR/推送限制设置
//	Registries        制品库: 包/标签/provenance
//	Missions          任务集与视图
//	KnowledgeBase     知识库
//	AI                AI 对话
//	Users             用户信息/邮箱/GPG
//	Workspace         云开发工作区
//	以及 Activities/Assets/Badge/Charge/Events/Followers/Labels/
//	MissionResources/NpcObservability/Rank/RepoCodeIssue/RepoContributor/
//	Search/Security/ArtifactSecurity/Starring
//
// # 约定
//
//   - 所有方法第一个参数为 context.Context, 返回 (结果, *Response, error).
//   - 请求体 (Form) 与查询参数 (Options) 字段为指针 + omitempty:
//     nil 不发送; 用 [Ptr] 构造.
//   - 列表接口的 Options 内嵌 [ListOptions] 分页参数;
//     用 [EachPage] 逐页遍历.
//   - 非 2xx/3xx 返回 [*ErrorResponse], 内含 errcode/errmsg/errparam.
//   - 无 JSON schema 的下载接口只返回 *Response,
//     响应体已缓冲: resp.Body 读一次, [Response.BodyBytes] 可多次取.
//   - Issue/PR 路径参数 number 为 int, 但模型中 Number 字段为 string
//     (CNB API 原样如此).
package cnb
