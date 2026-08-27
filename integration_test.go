package cnb

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"
)

// 真实 API 集成测试: 设置环境变量 CNB_TOKEN 后才会运行, 平时自动跳过.
//
//	CNB_TOKEN=你的访问令牌 go test -run Integration -v .
//
// 只使用「自身数据」接口 (不依赖特定仓库/组织存在), 任何有效令牌都能跑.
func integrationClient(t *testing.T) *Client {
	t.Helper()
	token := os.Getenv("CNB_TOKEN")
	if token == "" {
		t.Skip("需要环境变量 CNB_TOKEN (https://docs.cnb.cool/zh/develops/access-token.html)")
	}
	c, err := NewClient(token)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestIntegrationUserInfo(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	me, _, err := c.Users.GetUserInfo(ctx)
	if err != nil {
		t.Fatalf("GetUserInfo: %v", err)
	}
	if me.Username == "" {
		t.Error("username 为空")
	}
	t.Logf("当前用户: %s", me.Username)
}

func TestIntegrationListGroups(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	groups, resp, err := c.Organizations.ListTopGroups(ctx, &ListTopGroupsOptions{
		ListOptions: ListOptions{Page: 1, PageSize: 5},
	})
	if err != nil {
		t.Fatalf("ListTopGroups: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
	for _, g := range groups {
		t.Logf("组织: %s (path=%s role=%s)", g.Name, g.Path, g.AccessRole)
	}
}

func TestIntegrationEachPageIssues(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 分页遍历「我的 Issue」, 只取前 2 页验证翻页逻辑
	opts := &ListUserIssuesOptions{ListOptions: ListOptions{PageSize: 5}}
	total, pages := 0, 0
	stop := errors.New("stop iteration")
	err := EachPage(5, func(page int) ([]*UserIssue, error) {
		opts.Page = page
		items, _, err := c.Issues.ListUserIssues(ctx, opts)
		return items, err
	}, func(items []*UserIssue) error {
		total += len(items)
		pages++
		if pages >= 2 {
			return stop
		}
		return nil
	})
	if err != nil && !errors.Is(err, stop) {
		t.Fatalf("EachPage: %v", err)
	}
	t.Logf("翻页正常: %d 条 / %d 页", total, pages)
}

func TestIntegrationNotFound(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 一个确定不存在的仓库: 应得到结构化 404
	_, _, err := c.Repositories.GetByID(ctx, "definitely-not-exist-org-9x/repo")
	var apiErr *ErrorResponse
	if !AsErrorResponse(err, &apiErr) {
		t.Fatalf("want *ErrorResponse, got %#v", err)
	}
	t.Logf("404 结构化: status=%d errcode=%d msg=%.50s",
		apiErr.Response.StatusCode, apiErr.ErrCode, apiErr.ErrMsg)
}

func TestIntegrationGetRaw(t *testing.T) {
	c := integrationClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// GetRaw 双路径验证: JSON 字符串或裸文本都应成功
	raw, resp, err := c.Git.GetRaw(ctx, "cnb/awesome-cnb", "main/README.md", nil)
	if err != nil {
		var apiErr *ErrorResponse
		if AsErrorResponse(err, &apiErr) && apiErr.IsNotFound() {
			t.Skip("示例仓库不存在, 跳过")
		}
		t.Fatalf("GetRaw: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if raw != nil {
		t.Logf("GetRaw 前 40 字符: %.40s", *raw)
	}
}
