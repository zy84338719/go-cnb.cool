package cnb

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// 每个 HTTP 动词的请求构造与响应解码.
func TestHTTPVerbs(t *testing.T) {
	type call struct{ method, path string }
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, call{r.Method, r.URL.Path})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"path":"` + r.URL.Path + `"}`))
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL))
	ctx := context.Background()

	// GET
	if _, _, err := c.Repositories.GetByID(ctx, "a/b"); err != nil {
		t.Fatalf("GET: %v", err)
	}
	// POST (201)
	if _, _, err := c.Issues.CreateIssue(ctx, "a/b", PostIssueForm{Title: Ptr("x")}); err != nil {
		t.Fatalf("POST: %v", err)
	}
	// PATCH
	if _, _, err := c.Issues.UpdateIssue(ctx, "a/b", 1, PatchIssueForm{Title: Ptr("x")}); err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	// PUT
	if _, err := c.GitSettings.PutPullRequestSettings(ctx, "a/b", PullRequestSettings{}); err != nil {
		t.Fatalf("PUT: %v", err)
	}
	// DELETE
	if _, err := c.Issues.DeleteIssueLabels(ctx, "a/b", 1); err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	// HEAD (Releases 下载 HEAD 接口)
	if _, err := c.Releases.HeadReleasesAsset(ctx, "a/b", "v1", "f.zip"); err != nil {
		t.Fatalf("HEAD: %v", err)
	}

	want := []call{
		{"GET", "/a/b"}, {"POST", "/a/b/-/issues"}, {"PATCH", "/a/b/-/issues/1"},
		{"PUT", "/a/b/-/settings/pull-request"}, {"DELETE", "/a/b/-/issues/1/labels"},
		{"HEAD", "/a/b/-/releases/download/v1/f.zip"},
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %d, want %d: %+v", len(calls), len(want), calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call[%d] = %+v, want %+v", i, calls[i], want[i])
		}
	}
}

// 请求体确实是 JSON 且 Content-Type 正确.
func TestRequestBodyJSON(t *testing.T) {
	var body []byte
	var ctype string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		ctype = r.Header.Get("Content-Type")
		_, _ = w.Write([]byte("null"))
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL))
	_, _, _ = c.Issues.CreateIssue(context.Background(), "a/b", PostIssueForm{
		Title:  Ptr("标题"),
		Labels: []string{"l1", "l2"},
	})
	if ctype != "application/json" {
		t.Errorf("Content-Type = %q", ctype)
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("请求体不是合法 JSON: %v (%s)", err, body)
	}
	if m["title"] != "标题" {
		t.Errorf("title = %v", m["title"])
	}
	if labels, _ := m["labels"].([]any); len(labels) != 2 {
		t.Errorf("labels = %v", m["labels"])
	}
}

// 201 / 204 / 空 200 响应体.
func TestStatusVariants(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/groups": // CreateOrganization
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/a/b/-/git/tags/v1": // DeleteTag
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Path == "/emptyok":
			_, _ = w.Write(nil)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL))
	ctx := context.Background()

	// 201 + JSON body
	if _, err := c.Organizations.CreateOrganization(ctx, CreateGroupReq{Path: Ptr("x")}); err != nil {
		t.Fatalf("201: %v", err)
	}
	// 204 无 body
	if _, err := c.Git.DeleteTag(ctx, "a/b", "v1"); err != nil {
		t.Fatalf("204: %v", err)
	}
	// 空 200: v 非 nil 但 body 为空 => 不解码, 不报错
	req, _ := c.NewRequest(http.MethodGet, "/emptyok", nil)
	if _, err := c.Do(ctx, req, &struct{}{}); err != nil {
		t.Errorf("空 200 响应体不应报错: %v", err)
	}
}

// 302 重定向自动跟随 (Release/commit 附件预签名下载).
func TestRedirectFollowed(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("FILE-CONTENT"))
	}))
	defer final.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/obj", http.StatusFound)
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL))
	resp, err := c.Releases.GetReleasesAsset(context.Background(), "a/b", "v1", "f.zip", nil)
	if err != nil {
		t.Fatalf("302: %v", err)
	}
	data, _ := io.ReadAll(resp.Body)
	if string(data) != "FILE-CONTENT" {
		t.Errorf("重定向后内容 = %q", data)
	}
}

// 无 schema 接口: body 已缓冲, Body 流可读一次, BodyBytes 可多次取.
func TestRawBodyReReadable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("raw-bytes-abc"))
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL))
	resp, err := c.Git.GetArchive(context.Background(), "a/b", "main")
	if err != nil {
		t.Fatal(err)
	}
	d1, _ := io.ReadAll(resp.Body)
	if string(d1) != "raw-bytes-abc" {
		t.Fatalf("流读 = %q", d1)
	}
	// 流已读尽, 但缓冲副本随时可取、可多次取
	d2, _ := resp.BodyBytes()
	d3, _ := resp.BodyBytes()
	if string(d2) != "raw-bytes-abc" || string(d3) != "raw-bytes-abc" {
		t.Errorf("BodyBytes = %q / %q", d2, d3)
	}
	// 副本修改不影响内部缓冲
	d3[0] = 'X'
	d4, _ := resp.BodyBytes()
	if string(d4) != "raw-bytes-abc" {
		t.Errorf("副本应独立: %q", d4)
	}
}

// JSON 接口出错时, resp.Body 仍可读 (便于排查).
func TestErrorBodyStillReadable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errcode":1,"errmsg":"bad"}`))
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL))
	req, _ := c.NewRequest(http.MethodGet, "/x", nil)
	var v map[string]any
	resp, err := c.Do(context.Background(), req, &v)
	if err == nil {
		t.Fatal("want error")
	}
	if resp == nil {
		t.Fatal("error 时 resp 不应为 nil")
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "errcode") {
		t.Errorf("error body 不可读: %q", b)
	}
}

// context 取消.
func TestContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("null"))
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := c.Users.GetUserInfo(ctx)
	if err == nil {
		t.Fatal("取消的 ctx 应报错")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// 网络错误包装.
func TestRequestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL), WithHTTPClient(&http.Client{Timeout: 50 * time.Millisecond}))
	_, _, err := c.Users.GetUserInfo(context.Background())
	if err == nil {
		t.Fatal("超时应报错")
	}
}

// 空 token 不发 Authorization; 已设置的 Authorization 不被覆盖.
func TestAuthorizationHeader(t *testing.T) {
	var auths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auths = append(auths, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte("null"))
	}))
	defer srv.Close()

	// 空 token
	c1, _ := NewClient("", WithBaseURL(srv.URL))
	req, _ := c1.NewRequest(http.MethodGet, "/x", nil)
	if _, err := c1.Do(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	if auths[0] != "" {
		t.Errorf("空 token 不应发 Authorization, got %q", auths[0])
	}

	// 预置 Authorization 不被覆盖
	c2, _ := NewClient("tok", WithBaseURL(srv.URL))
	req2, _ := c2.NewRequest(http.MethodGet, "/x", nil)
	req2.Header.Set("Authorization", "Bearer custom")
	if _, err := c2.Do(context.Background(), req2, nil); err != nil {
		t.Fatal(err)
	}
	if auths[1] != "Bearer custom" {
		t.Errorf("预置 Authorization 被覆盖: %q", auths[1])
	}
}

// 绝对 URL NewRequest + 自定义 UserAgent/Accept.
func TestNewRequestAbsoluteAndHeaders(t *testing.T) {
	var ua, accept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua, accept = r.Header.Get("User-Agent"), r.Header.Get("Accept")
		_, _ = w.Write([]byte("null"))
	}))
	defer srv.Close()

	c, _ := NewClient("t", WithBaseURL("http://127.0.0.1:9/"), WithHTTPClient(http.DefaultClient))
	c.UserAgent = "my-agent/1.0"
	c.Accept = "application/vnd.cnb.api+json"
	req, err := c.NewRequest(http.MethodGet, srv.URL+"/abs", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Do(context.Background(), req, nil); err != nil {
		t.Fatal(err)
	}
	if ua != "my-agent/1.0" || accept != "application/vnd.cnb.api+json" {
		t.Errorf("UA=%q Accept=%q", ua, accept)
	}
}

// 响应 JSON 非法时报错且带状态码.
func TestInvalidJSONResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	c, _ := NewClient("t", WithBaseURL(srv.URL))
	_, _, err := c.Users.GetUserInfo(context.Background())
	if err == nil {
		t.Fatal("非法 JSON 应报错")
	}
	if !strings.Contains(err.Error(), "status 200") {
		t.Errorf("错误应包含状态码: %v", err)
	}
}
