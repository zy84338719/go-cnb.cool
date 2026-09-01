package cnb

// ListOptions 是分页接口的公共查询参数, 内嵌在各 *XxxOptions 中.
//
// CNB 分页: page 从 1 开始, page_size 缺省 10, 上限 100.
type ListOptions struct {
	Page     int `json:"page,omitempty"`
	PageSize int `json:"page_size,omitempty"`
}

// EachPage 逐页拉取列表, 直至返回空页/不足一页, 或 fn 返回错误.
//
// 用法 (配合生成的 Options, 注意在闭包里更新 Page):
//
//	opts := &cnb.ListIssuesOptions{ListOptions: cnb.ListOptions{PageSize: 100}}
//	err := cnb.EachPage(100, func(page int) ([]*cnb.Issue, error) {
//	    opts.Page = page
//	    issues, _, err := client.Issues.ListIssues(ctx, "org/repo", opts)
//	    return issues, err
//	}, func(issues []*cnb.Issue) error {
//	    for _, is := range issues {
//	        fmt.Println(is.Title)
//	    }
//	    return nil
//	})
//
// 服务端未返回总数, 终止条件依赖"当前页条数 < pageSize".
// 注意: pageSize 必须与传给 Options 的 PageSize 一致; CNB 服务端将
// page_size 钳制在 1~100, 传入超过 100 的值会被自动收缩到 100 ——
// 此时请确保 Options.PageSize 同样为 100, 否则会因页大小不一致提前终止.
func EachPage[T any](pageSize int, fetch func(page int) ([]T, error), fn func(items []T) error) error {
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100 // 服务端上限, 与终止条件保持一致
	}
	for page := 1; ; page++ {
		items, err := fetch(page)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		if err := fn(items); err != nil {
			return err
		}
		if len(items) < pageSize {
			return nil
		}
	}
}
