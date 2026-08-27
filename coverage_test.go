package cnb

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

//go:embed internal/gen/swagger.json
var swaggerSpec []byte

type specFile struct {
	Paths map[string]map[string]specOp `json:"paths"`
}

type specOp struct {
	OperationID string         `json:"operationId"`
	Tags        []string       `json:"tags"`
	Parameters  []specParam    `json:"parameters"`
	Responses   map[string]any `json:"responses"`
}

type specParam struct {
	Name string `json:"name"`
	In   string `json:"in"`
	Type string `json:"type"` // string | integer
}

var (
	onceSpec    sync.Once
	parsedSpec  *specFile
	specOps     []specOpWithPath
	parseSpecEr error
)

type specOpWithPath struct {
	specOp
	Method string
	Path   string
}

func loadSpec(t *testing.T) []specOpWithPath {
	t.Helper()
	onceSpec.Do(func() {
		parsedSpec = &specFile{}
		if err := json.Unmarshal(swaggerSpec, parsedSpec); err != nil {
			parseSpecEr = err
			return
		}
		for path, item := range parsedSpec.Paths {
			for method, op := range item {
				if op.OperationID == "" {
					continue
				}
				specOps = append(specOps, specOpWithPath{op, strings.ToUpper(method), path})
			}
		}
	})
	if parseSpecEr != nil {
		t.Fatalf("parse embedded swagger.json: %v", parseSpecEr)
	}
	if len(specOps) == 0 {
		t.Fatal("no operations parsed from embedded swagger.json")
	}
	return specOps
}

// serviceMethods 反射收集 Client 上所有 Service 的全部方法.
func serviceMethods(t *testing.T, c *Client) map[string]reflect.Value {
	t.Helper()
	methods := make(map[string]reflect.Value)
	v := reflect.ValueOf(c).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() != reflect.Ptr || f.IsNil() {
			continue
		}
		if !strings.HasSuffix(f.Type().Elem().Name(), "Service") {
			continue
		}
		// 方法均为指针接收者, 必须从指针类型取方法集
		mt := f.Type()
		for j := 0; j < mt.NumMethod(); j++ {
			m := mt.Method(j)
			methods[m.Name] = f.Method(m.Index)
		}
	}
	return methods
}

// TestRouteTableFullCoverage 是核心回归测试: 把 swagger.json 里全部 259 个操作
// 逐一通过 SDK 真实发出请求, 断言:
//  1. 每个 operationId 都有对应的 SDK 方法
//  2. HTTP 动词与 spec 一致
//  3. 请求路径 (含路径参数填充顺序) 与 spec 模板一致
//  4. 零值参数调用全程不 panic、不报错
func TestRouteTableFullCoverage(t *testing.T) {
	ops := loadSpec(t)

	var (
		mu       sync.Mutex
		gotReq   chan struct{} = make(chan struct{}, 1) // 串行化用
		lastM    string
		lastPath string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		lastM, lastPath = r.Method, r.URL.Path
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		// 统一返回 JSON null: 对指针/切片/RawMessage 目标均可安全解码
		_, _ = w.Write([]byte("null"))
		gotReq <- struct{}{}
	}))
	defer srv.Close()

	client, err := NewClient("test-token", WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	methods := serviceMethods(t, client)

	if got, want := len(methods), len(ops); got < want {
		t.Fatalf("SDK 方法数 %d < spec 操作数 %d", got, want)
	}

	var missing, wrongMethod, wrongPath, callErr int
	for _, op := range ops {
		m, ok := methods[op.OperationID]
		if !ok {
			missing++
			t.Errorf("缺少方法: %s (%s %s, tag=%v)", op.OperationID, op.Method, op.Path, op.Tags)
			continue
		}

		args, expectedPath := buildCallArgs(t, m, op)
		results := m.Call(args)
		if errVal := results[len(results)-1]; !errVal.IsNil() {
			callErr++
			t.Errorf("%s 调用出错: %v", op.OperationID, errVal.Interface())
			continue
		}
		<-gotReq

		mu.Lock()
		gm, gp := lastM, lastPath
		mu.Unlock()
		if gm != op.Method {
			wrongMethod++
			t.Errorf("%s: HTTP 动词 = %s, want %s", op.OperationID, gm, op.Method)
		}
		if gp != expectedPath {
			wrongPath++
			t.Errorf("%s: 路径 = %q, want %q (模板 %q)", op.OperationID, gp, expectedPath, op.Path)
		}
	}
	if missing+wrongMethod+wrongPath+callErr > 0 {
		t.Fatalf("路由表校验失败: 缺失 %d, 动词错 %d, 路径错 %d, 调用错 %d (共 %d 操作)",
			missing, wrongMethod, wrongPath, callErr, len(ops))
	}
	t.Logf("路由表全量校验通过: %d 个操作, 方法总数 %d", len(ops), len(methods))
}

// buildCallArgs 按生成规则 (路径参数..., opts, body) 构造零值实参,
// 并返回期望的请求路径 (spec 模板按参数名替换).
func buildCallArgs(t *testing.T, m reflect.Value, op specOpWithPath) ([]reflect.Value, string) {
	t.Helper()
	mt := m.Type()

	args := []reflect.Value{reflect.ValueOf(context.Background())}
	pathVals := map[string]string{}
	argIdx := 1
	expected := op.Path

	for _, p := range op.Parameters {
		if p.In != "path" {
			continue
		}
		var val string
		switch p.Type {
		case "integer":
			if mt.In(argIdx).Kind() != reflect.Int {
				t.Fatalf("%s: 参数 %s 期望 int, 实际 %s", op.OperationID, p.Name, mt.In(argIdx).Kind())
			}
			args = append(args, reflect.ValueOf(1))
			val = "1"
		default:
			if mt.In(argIdx).Kind() != reflect.String {
				t.Fatalf("%s: 参数 %s 期望 string, 实际 %s", op.OperationID, p.Name, mt.In(argIdx).Kind())
			}
			val = "pv" + strings.ToUpper(p.Name[:1]) + p.Name[1:]
			args = append(args, reflect.ValueOf(val))
		}
		pathVals[p.Name] = val
		argIdx++
		expected = strings.ReplaceAll(expected, "{"+p.Name+"}", val)
	}

	// 生成的参数顺序: path 参数之后, 有 query 参数则跟 opts, 再跟 body (若有)
	hasQuery, hasBody := false, false
	for _, p := range op.Parameters {
		switch p.In {
		case "query":
			hasQuery = true
		case "body":
			hasBody = true
		}
	}
	if hasQuery {
		if mt.In(argIdx).Kind() != reflect.Ptr {
			t.Fatalf("%s: 第 %d 个参数应为 *Options", op.OperationID, argIdx)
		}
		args = append(args, reflect.Zero(mt.In(argIdx)))
		argIdx++
	}
	if hasBody {
		bt := mt.In(argIdx)
		var bv reflect.Value
		if bt.Kind() == reflect.Ptr {
			bv = reflect.Zero(bt)
		} else {
			bv = reflect.New(bt).Elem() // 零值表单
		}
		args = append(args, bv)
		argIdx++
	}
	if argIdx != mt.NumIn() {
		t.Fatalf("%s: 参数个数不匹配: SDK 需要 %d 个, 测试构造了 %d 个",
			op.OperationID, mt.NumIn(), argIdx)
	}
	return args, expected
}

// TestOperationCount 锁住 API 覆盖规模, 防止 spec 更新后静默缩水.
func TestOperationCount(t *testing.T) {
	ops := loadSpec(t)
	if len(ops) != 259 {
		t.Errorf("spec 操作数 = %d, 期望 259 (CNB API 更新后请同步更新此断言)", len(ops))
	}
}

// TestMethodsHaveContext 确保 SDK 一致性: 所有方法第一个参数是 context.Context.
func TestMethodsHaveContext(t *testing.T) {
	client, err := NewClient("t")
	if err != nil {
		t.Fatal(err)
	}
	methods := serviceMethods(t, client)
	bad := 0
	for name, m := range methods {
		mt := m.Type()
		if mt.NumIn() < 1 || mt.In(0) != reflect.TypeOf((*context.Context)(nil)).Elem() {
			t.Errorf("方法 %s 第一参数不是 context.Context", name)
			bad++
		}
		if mt.NumOut() < 2 {
			t.Errorf("方法 %s 返回值少于 2 个", name)
			bad++
		}
	}
	if bad > 0 {
		t.Fatalf("%d 个方法签名不符合约定", bad)
	}
	_ = fmt.Sprint()
}
