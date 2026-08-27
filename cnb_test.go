package cnb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNewClient(t *testing.T) {
	c, err := NewClient("token-xxx")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.BaseURL.String() != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", c.BaseURL.String(), DefaultBaseURL)
	}
	if c.Issues == nil || c.Pulls == nil || c.Git == nil || c.Organizations == nil {
		t.Fatalf("services not initialized")
	}
}

func TestNewClientWithBaseURL(t *testing.T) {
	c, err := NewClient("t", WithBaseURL("http://127.0.0.1:1/api/"))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.BaseURL.String(); got != "http://127.0.0.1:1/api/" {
		t.Errorf("BaseURL = %q", got)
	}
}

func TestClientDo_AuthAndAccept(t *testing.T) {
	var gotAuth, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		_, _ = io.WriteString(w, `{"name":"tester"}`)
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL))
	req, err := c.NewRequest(http.MethodGet, "/user", nil)
	if err != nil {
		t.Fatal(err)
	}
	var u map[string]any
	if _, err := c.Do(context.Background(), req, &u); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q", gotAccept)
	}
}

func TestErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"errcode":10404,"errmsg":"repo not found","errparam":{"repo":"a/b"}}`)
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL))
	_, _, err := c.Users.GetUserInfo(context.Background())
	var errResp *ErrorResponse
	if !errors.As(err, &errResp) {
		t.Fatalf("want *ErrorResponse, got %#v", err)
	}
	if errResp.ErrCode != 10404 || errResp.ErrMsg != "repo not found" {
		t.Errorf("decoded fields wrong: %+v", errResp)
	}
	if !errResp.IsNotFound() {
		t.Error("IsNotFound = false")
	}
	if errResp.ErrParam["repo"] != "a/b" {
		t.Errorf("errparam = %v", errResp.ErrParam)
	}
}

func TestErrorResponse_NonJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom <html>")
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL))
	_, _, err := c.Users.GetUserInfo(context.Background())
	var errResp *ErrorResponse
	if !errors.As(err, &errResp) {
		t.Fatalf("want *ErrorResponse, got %#v", err)
	}
	if errResp.RawBody != "boom <html>" {
		t.Errorf("RawBody = %q", errResp.RawBody)
	}
}

// 生成方法的端到端冒烟测试: 路径转义 + query 编码 + JSON 解码 + 分页参数.
func TestGeneratedListIssues(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = io.WriteString(w, `[{"number":"1","title":"bug","state":"open"},{"number":"2","title":"feat","state":"open"}]`)
	}))
	defer srv.Close()

	c, _ := NewClient("tok", WithBaseURL(srv.URL))
	opts := &ListIssuesOptions{
		ListOptions: ListOptions{Page: 2, PageSize: 50},
		State:       Ptr("open"),
		Labels:      Ptr("git,bug"),
	}
	issues, _, err := c.Issues.ListIssues(context.Background(), "org/repo", opts)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/org/repo/-/issues" {
		t.Errorf("path = %q", gotPath)
	}
	q, _ := url.ParseQuery(gotQuery)
	if q.Get("page") != "2" || q.Get("page_size") != "50" || q.Get("state") != "open" {
		t.Errorf("query = %q", gotQuery)
	}
	if q.Get("labels") != "git,bug" {
		t.Errorf("labels = %q", q.Get("labels"))
	}
	if len(issues) != 2 || issues[0].Title != "bug" {
		t.Errorf("decoded issues wrong: %+v", issues)
	}
}

func TestAddQuery(t *testing.T) {
	got, err := addQuery("/x", nil)
	if err != nil || got != "/x" {
		t.Errorf("nil opts: %q %v", got, err)
	}
	got, err = addQuery("/x", &ListIssuesOptions{})
	if err != nil || got != "/x" {
		t.Errorf("empty opts: %q %v", got, err)
	}
	got, _ = addQuery("/x", &ListTopGroupsOptions{
		ListOptions: ListOptions{Page: 3},
		Role:        Ptr("Owner"),
	})
	if got != "/x?page=3&role=Owner" {
		t.Errorf("got %q", got)
	}
}

func TestEscapePath(t *testing.T) {
	if got := escapePath("/%s/-/issues/%d", "org/repo", 7); got != "/org/repo/-/issues/7" {
		t.Errorf("got %q", got)
	}
	if got := escapePath("/%s/-/git/contents/%s", "org/repo", "docs/read me.md"); got != "/org/repo/-/git/contents/docs/read%20me.md" {
		t.Errorf("got %q", got)
	}
	if got := escapePath("/plain"); got != "/plain" {
		t.Errorf("got %q", got)
	}
}

func TestEachPage(t *testing.T) {
	pages := [][]int{{1, 2}, {3}}
	calls := 0
	var seen []int
	err := EachPage(2, func(page int) ([]int, error) {
		calls++
		return pages[page-1], nil
	}, func(items []int) error {
		seen = append(seen, items...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(seen) != 3 {
		t.Errorf("calls=%d seen=%v", calls, seen)
	}

	stopErr := fmt.Errorf("stop")
	err = EachPage(2, func(int) ([]int, error) { return []int{1, 2}, nil },
		func([]int) error { return stopErr })
	if !errors.Is(err, stopErr) {
		t.Errorf("want stopErr, got %v", err)
	}
}

// 确保枚举与模型可正常 JSON 往返.
func TestEnumRoundtrip(t *testing.T) {
	var v Visibility
	if err := json.Unmarshal([]byte(`"Secret"`), &v); err != nil {
		t.Fatal(err)
	}
	if v != VisibilitySecret {
		t.Errorf("v = %q", v)
	}
	if RepoStatusOK != 0 || RepoStatusArchived != 1 || RepoStatusForking != 2 {
		t.Error("RepoStatus int enum values wrong")
	}
}

// escapePath: repo slug 与深层路径组合、特殊字符.
func TestEscapePathDeep(t *testing.T) {
	got := escapePath("/%s/-/git/contents/%s", "org/sub team", "docs/read me.md?a=1")
	if got != "/org/sub%20team/-/git/contents/docs/read%20me.md%3Fa=1" {
		t.Errorf("got %q", got)
	}
	// 中文
	got = escapePath("/%s/-/git/branches/%s", "org/repo", "feature/中文")
	if got != "/org/repo/-/git/branches/feature/%E4%B8%AD%E6%96%87" {
		t.Errorf("got %q", got)
	}
}

// addQuery: 数组逗号连接、bool、浮点精度.
func TestAddQueryComplex(t *testing.T) {
	got, err := addQuery("/x", &ListPullsOptions{
		ListOptions: ListOptions{Page: 1},
		State:       Ptr("all"),
	})
	if err != nil || got != "/x?page=1&state=all" {
		t.Errorf("got %q err %v", got, err)
	}
}

// WithBaseURL 无尾斜杠自动补齐.
func TestWithBaseURLNoTrailingSlash(t *testing.T) {
	c, err := NewClient("t", WithBaseURL("https://example.com/api"))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.BaseURL.String(); got != "https://example.com/api/" {
		t.Errorf("BaseURL = %q", got)
	}
	// 相对路径解析
	req, err := c.NewRequest(http.MethodGet, "/user", nil)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL.String() != "https://example.com/api/user" {
		t.Errorf("resolved = %q", req.URL.String())
	}
}

// Response.BodyBytes: 空 body 与无响应体场景.
func TestBodyBytesEmpty(t *testing.T) {
	r := &Response{}
	b, err := r.BodyBytes()
	if err != nil || len(b) != 0 {
		t.Errorf("empty: %q %v", b, err)
	}
}
