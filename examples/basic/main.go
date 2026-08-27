// cnb SDK 基础用法示例.
//
// 运行:
//
//	export CNB_TOKEN="你的访问令牌"   # https://docs.cnb.cool/zh/develops/access-token.html
//	go run ./examples/basic
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	cnb "github.com/zy84338719/go-cnb.cool"
)

func main() {
	token := os.Getenv("CNB_TOKEN")
	if token == "" {
		fmt.Println("请先设置 CNB_TOKEN 环境变量")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := cnb.NewClient(token)
	if err != nil {
		panic(err)
	}

	// 1. 当前用户信息
	user, _, err := client.Users.GetUserInfo(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Printf("当前用户: %s\n", user.Username)

	// 2. 有权限的顶层组织 (带查询参数)
	groups, _, err := client.Organizations.ListTopGroups(ctx, &cnb.ListTopGroupsOptions{
		ListOptions: cnb.ListOptions{Page: 1, PageSize: 5},
	})
	if err != nil {
		panic(err)
	}
	for _, g := range groups {
		fmt.Printf("组织: %s (成员 %d, 仓库 %d)\n", g.Name, g.AllMemberCount, g.AllSubRepoCount)
	}

	// 3. 逐页遍历某仓库的 Issue
	opts := &cnb.ListIssuesOptions{ListOptions: cnb.ListOptions{PageSize: 20}}
	err = cnb.EachPage(20, func(page int) ([]*cnb.Issue, error) {
		opts.Page = page
		issues, _, err := client.Issues.ListIssues(ctx, "cnb/awesome-cnb", opts)
		return issues, err
	}, func(issues []*cnb.Issue) error {
		for _, is := range issues {
			fmt.Printf("#%s %s [%s]\n", is.Number, is.Title, is.State)
		}
		return nil
	})
	if err != nil {
		var apiErr *cnb.ErrorResponse
		if cnb.AsErrorResponse(err, &apiErr) && apiErr.IsNotFound() {
			fmt.Println("示例仓库不存在, 跳过 Issue 遍历")
		} else {
			panic(err)
		}
	}
}
