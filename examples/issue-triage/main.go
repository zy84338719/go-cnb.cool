// issue 分诊示例: 遍历仓库 open issue, 给没有标签的打上 "triage" 标签并留评论.
//
// 运行:
//
//	export CNB_TOKEN="你的访问令牌"
//	export TRIAGE_REPO="org/repo"
//	go run ./examples/issue-triage
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	cnb "github.com/zy84338719/go-cnb.cool"
)

func main() {
	token := os.Getenv("CNB_TOKEN")
	repo := os.Getenv("TRIAGE_REPO")
	if token == "" || repo == "" {
		fmt.Println("请先设置 CNB_TOKEN 与 TRIAGE_REPO 环境变量")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := cnb.NewClient(token)
	if err != nil {
		panic(err)
	}

	triaged, skipped := 0, 0
	opts := &cnb.ListIssuesOptions{
		ListOptions: cnb.ListOptions{PageSize: 50},
		State:       cnb.Ptr("open"),
	}
	err = cnb.EachPage(50, func(page int) ([]*cnb.Issue, error) {
		opts.Page = page
		issues, _, err := client.Issues.ListIssues(ctx, repo, opts)
		return issues, err
	}, func(issues []*cnb.Issue) error {
		for _, is := range issues {
			if len(is.Labels) > 0 {
				skipped++
				continue // 已有标签, 不处理
			}
			// 注意: 模型中 Number 是 string, 路径参数 number 是 int (CNB API 如此定义)
			num, err := strconv.Atoi(is.Number)
			if err != nil {
				return fmt.Errorf("issue number %q 非 数字: %w", is.Number, err)
			}
			// 打标签 (PostIssueLabels)
			_, _, err = client.Issues.PostIssueLabels(ctx, repo, num, cnb.PostIssueLabelsForm{
				Labels: []string{"triage"},
			})
			if err != nil {
				var apiErr *cnb.ErrorResponse
				if cnb.AsErrorResponse(err, &apiErr) && apiErr.IsForbidden() {
					return errors.New("令牌缺少 repo-issue 写权限: " + apiErr.Error())
				}
				return fmt.Errorf("issue #%s 打标签失败: %w", is.Number, err)
			}
			// 留评论
			_, _, err = client.Issues.PostIssueComment(ctx, repo, num, cnb.PostIssueCommentForm{
				Body: cnb.Ptr("暂无标签, 已自动标记为 triage, 请相关同学认领处理。"),
			})
			if err != nil {
				return fmt.Errorf("issue #%s 评论失败: %w", is.Number, err)
			}
			triaged++
			fmt.Printf("#%s %s -> triage\n", is.Number, is.Title)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("完成: 新分诊 %d, 已有标签跳过 %d\n", triaged, skipped)
}
