package cnb

import (
	"encoding/json"
	"testing"
)

// 枚举常量值与 CNB API 实际取值锁定.
func TestEnumConstantValues(t *testing.T) {
	strEnums := []struct {
		got  string
		want string
	}{
		{string(AccessRoleAnonymous), "Unknown"},
		{string(AccessRoleOwner), "Owner"},
		{string(VisibilityPrivate), "Private"},
		{string(VisibilitySecret), "Secret"},
		{string(AssetRecordTypeIssueImg), "issue_img"},
		{string(PackageTypeArtifactory), "all"},
		{string(PackageTypeDocker), "docker"},
	}
	for _, c := range strEnums {
		if c.got != c.want {
			t.Errorf("string 枚举常量 = %q, want %q", c.got, c.want)
		}
	}
	intEnums := []struct {
		got  int
		want int
	}{
		{int(RepoStatusOK), 0},
		{int(RepoStatusArchived), 1},
		{int(RepoStatusForking), 2},
		{int(UserTypeWeChatUser), 0},
		{int(UserTypeRobotUser), 3},
	}
	for _, c := range intEnums {
		if c.got != c.want {
			t.Errorf("int 枚举常量 = %d, want %d", c.got, c.want)
		}
	}
}

// 枚举 JSON 往返: wire 值 <-> 具名类型.
func TestEnumJSONRoundtrip(t *testing.T) {
	in := []Visibility{VisibilityPrivate, VisibilityPublic, VisibilitySecret}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `["Private","Public","Secret"]` {
		t.Errorf("marshal = %s", b)
	}
	var out []Visibility
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("out[%d] = %v, want %v", i, out[i], in[i])
		}
	}

	// 无效值: 解码不报错, 保持零值 (宽松策略, 服务端新增枚举值时不炸)
	var v Visibility
	if err := json.Unmarshal([]byte(`"BrandNew"`), &v); err != nil {
		t.Fatalf("unknown enum value should not fail: %v", err)
	}
}

// 响应模型: 值字段, 完整解码.
func TestResponseModelDecode(t *testing.T) {
	raw := `{
		"number": "42",
		"title": "标题",
		"state": "open",
		"labels": [{"name": "bug"}, {"name": "feature"}],
		"author": {"username": "alice", "name": "Alice"},
		"created_at": "2026-08-27 10:00:00"
	}`
	var is Issue
	if err := json.Unmarshal([]byte(raw), &is); err != nil {
		t.Fatal(err)
	}
	if is.Number != "42" || is.Title != "标题" || is.State != "open" {
		t.Errorf("decoded: %+v", is)
	}
	if len(is.Labels) != 2 || is.Labels[0].Name != "bug" {
		t.Errorf("labels: %+v", is.Labels)
	}
	if is.Author.Username != "alice" {
		t.Errorf("author: %+v", is.Author)
	}

	// roundtrip: 序列化字段名与 wire 名一致
	b, err := json.Marshal(&is)
	if err != nil {
		t.Fatal(err)
	}
	var back Issue
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Number != is.Number || back.Title != is.Title || len(back.Labels) != 2 {
		t.Errorf("roundtrip lost fields: %s", b)
	}
}

// 响应模型: 服务端缺字段时零值安全.
func TestResponseModelPartialDecode(t *testing.T) {
	var is Issue
	if err := json.Unmarshal([]byte(`{"title":"only-title"}`), &is); err != nil {
		t.Fatal(err)
	}
	if is.Title != "only-title" || is.Number != "" || is.State != "" {
		t.Errorf("partial decode: %+v", is)
	}
	var org OrganizationAccess
	if err := json.Unmarshal([]byte(`{}`), &org); err != nil {
		t.Fatal(err)
	}
	if org.AccessRole != AccessRole("") || org.AllMemberCount != 0 {
		t.Errorf("empty decode: %+v", org)
	}
}

// 请求表单: 指针 + omitempty —— nil 字段绝不发送 (PATCH 语义安全).
func TestRequestFormOmitEmpty(t *testing.T) {
	b, err := json.Marshal(PatchIssueForm{Title: Ptr("新标题")})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m["title"] != "新标题" {
		t.Errorf("nil 字段应被省略, got %s", b)
	}

	// 显式空串会发送 (区别于 nil)
	b, _ = json.Marshal(PatchIssueForm{Title: Ptr("")})
	if string(b) != `{"title":""}` {
		t.Errorf("显式空串应发送, got %s", b)
	}

	// 零值表单 => 空对象
	b, _ = json.Marshal(MergePullRequest{})
	if string(b) != `{}` {
		t.Errorf("零值表单应为 {}, got %s", b)
	}
}

// 请求表单: 完整字段序列化 wire 名正确.
func TestRequestFormWireNames(t *testing.T) {
	b, err := json.Marshal(MergePullRequest{
		MergeStyle:    Ptr("squash"),
		CommitTitle:   Ptr("t"),
		CommitMessage: Ptr("m"),
		Force:         Ptr(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"commit_message":"m","commit_title":"t","force":true,"merge_style":"squash"}`
	if string(b) != want {
		t.Errorf("marshal = %s, want %s", b, want)
	}
}

// ListOptions 内嵌: JSON 序列化时字段被提升 (分页参数平铺).
func TestListOptionsInline(t *testing.T) {
	opts := ListIssuesOptions{
		ListOptions: ListOptions{Page: 3, PageSize: 20},
		State:       Ptr("closed"),
	}
	b, err := json.Marshal(&opts)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"page", "page_size", "state"} {
		if _, ok := m[k]; !ok {
			t.Errorf("缺少平铺字段 %q: %s", k, b)
		}
	}

	// 零分页不发送
	b, _ = json.Marshal(ListIssuesOptions{State: Ptr("open")})
	if string(b) != `{"state":"open"}` {
		t.Errorf("零值分页应省略, got %s", b)
	}
}
