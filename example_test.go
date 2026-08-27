package cnb_test

import (
	"context"
	"fmt"
	"io"
	"log"

	cnb "github.com/zy84338719/go-cnb.cool"
)

// 本文件把 README 中的全部用法示例做成可编译的 Example,
// 由 CI 保证文档示例与 SDK 实际签名始终一致 (同时展示在 pkg.go.dev).

func setupClient() *cnb.Client {
	client, err := cnb.NewClient("your-access-token")
	if err != nil {
		log.Fatal(err)
	}
	return client
}

func setupCtx() context.Context { return context.Background() }

func ExampleNewClient() {
	client, err := cnb.NewClient("your-access-token",
		cnb.WithHTTPClient(nil), // 使用默认 http.Client; 可换自定义
		cnb.WithBaseURL(""),     // 使用默认 https://api.cnb.cool/
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(client != nil)
}

func ExampleClient_users() {
	client := setupClient()
	_ = client // 完整用法见 README「快速开始」与 examples/basic
}

// Issue: 创建 / 更新 / 评论.
func ExampleIssuesService() {
	client := setupClient()
	ctx := setupCtx()

	issue, _, err := client.Issues.CreateIssue(ctx, "org/repo", cnb.PostIssueForm{
		Title:  cnb.Ptr("支持导出报表"),
		Labels: []string{"feature"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(issue.Number)

	_, _, err = client.Issues.UpdateIssue(ctx, "org/repo", 42, cnb.PatchIssueForm{
		State:       cnb.Ptr("closed"),
		StateReason: cnb.Ptr("completed"),
	})
	if err != nil {
		log.Fatal(err)
	}

	_, _, err = client.Issues.PostIssueComment(ctx, "org/repo", 42, cnb.PostIssueCommentForm{
		Body: cnb.Ptr("已修复, 请验证"),
	})
	if err != nil {
		log.Fatal(err)
	}
}

// Pull Request: 创建 / 合并.
func ExamplePullsService() {
	client := setupClient()
	ctx := setupCtx()

	pr, _, err := client.Pulls.PostPull(ctx, "org/repo", cnb.PullCreationForm{
		Title: cnb.Ptr("feat: 报表导出"),
		Head:  cnb.Ptr("feat/export"),
		Base:  cnb.Ptr("main"),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(pr.Number)

	merged, _, err := client.Pulls.MergePull(ctx, "org/repo", "12", cnb.MergePullRequest{
		MergeStyle: cnb.Ptr("squash"), // merge / squash / rebase
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(merged.Merged)
}

// Git: 分支 / 提交 / 文件内容 / 对比.
func ExampleGitService() {
	client := setupClient()
	ctx := setupCtx()

	_, _, _ = client.Git.ListBranches(ctx, "org/repo", &cnb.ListBranchesOptions{
		ListOptions: cnb.ListOptions{PageSize: 100},
	})

	_, _, _ = client.Git.ListCommits(ctx, "org/repo", &cnb.ListCommitsOptions{
		Sha:   cnb.Ptr("main"),
		Since: cnb.Ptr("2026-01-01T00:00:00Z"),
	})

	content, _, _ := client.Git.GetContent(ctx, "org/repo", "README.md", &cnb.GetContentOptions{
		Ref: cnb.Ptr("main"),
	})
	fmt.Println(content.Path)

	_, _, _ = client.Git.GetCompareCommits(ctx, "org/repo", "main...dev")
}

// 构建流水线: 触发 / 状态 / 日志.
func ExampleBuildService() {
	client := setupClient()
	ctx := setupCtx()

	build, _, err := client.Build.StartBuild(ctx, "org/repo", cnb.StartBuildReq{
		Branch: cnb.Ptr("main"),
		Event:  cnb.Ptr("api_trigger"),
		Env:    map[string]string{"FOO": "bar"},
	})
	if err != nil {
		log.Fatal(err)
	}

	_, _, _ = client.Build.GetBuildStatus(ctx, "org/repo", build.Sn)
	_, _, _ = client.Build.GetBuildLogs(ctx, "org/repo", &cnb.GetBuildLogsOptions{
		Event: cnb.Ptr("push"),
	})
}

// 制品库: 包与标签.
func ExampleRegistriesService() {
	client := setupClient()
	ctx := setupCtx()

	pkgs, _, err := client.Registries.ListPackages(ctx, "org", &cnb.ListPackagesOptions{
		Type: cnb.Ptr("docker"),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(pkgs))

	_, _, _ = client.Registries.ListPackageTags(ctx, "org", "docker", "my-app", nil)
}

// 组织与成员.
func ExampleOrganizationsService() {
	client := setupClient()
	ctx := setupCtx()

	_, err := client.Organizations.CreateOrganization(ctx, cnb.CreateGroupReq{
		Path:        cnb.Ptr("my-org"),
		Description: cnb.Ptr("我的组织"),
	})
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.Members.AddMembersOfGroup(ctx, "my-org", "someone", cnb.UpdateMembersRequest{
		AccessLevel: cnb.Ptr("Developer"),
	})
	if err != nil {
		log.Fatal(err)
	}
}

// 分页: EachPage 逐页遍历.
func ExampleEachPage() {
	client := setupClient()
	ctx := setupCtx()

	opts := &cnb.ListPullsOptions{ListOptions: cnb.ListOptions{PageSize: 100}}
	err := cnb.EachPage(100, func(page int) ([]*cnb.PullRequest, error) {
		opts.Page = page
		prs, _, err := client.Pulls.ListPulls(ctx, "org/repo", opts)
		return prs, err
	}, func(prs []*cnb.PullRequest) error {
		for _, pr := range prs {
			fmt.Println(pr.Title)
		}
		return nil // 返回 error 提前终止
	})
	if err != nil {
		log.Fatal(err)
	}
}

// 错误处理.
func ExampleAsErrorResponse() {
	client := setupClient()
	ctx := setupCtx()

	_, _, err := client.Issues.GetIssue(ctx, "org/repo", 42)
	if err != nil {
		var apiErr *cnb.ErrorResponse
		if cnb.AsErrorResponse(err, &apiErr) {
			fmt.Printf("errcode=%d errmsg=%s\n", apiErr.ErrCode, apiErr.ErrMsg)
			switch {
			case apiErr.IsUnauthorized(): // 401 令牌无效/过期
			case apiErr.IsForbidden(): // 403 令牌权限不足
			case apiErr.IsNotFound(): // 404 资源不存在
			}
		}
	}
}

// 归档与原始文件下载.
func ExampleGitService_getArchive() {
	client := setupClient()
	ctx := setupCtx()

	resp, err := client.Git.GetArchive(ctx, "org/repo", "main")
	if err != nil {
		log.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	_ = data // tar/zip 归档内容; 也可 resp.BodyBytes() 多次取
}

// 枚举常量 (响应模型字段).
func ExampleAccessRole() {
	group, _, _ := setupClient().Organizations.GetGroup(setupCtx(), "my-org")
	if group.AccessRole == cnb.AccessRoleOwner {
		fmt.Println("owner")
	}
}
