package xiaohongshu

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/proto"
)

type searchResponse struct {
	empty bool
	err   error
}

// watchSearchResponses 只观察当前关键词的第一页，不重放请求、不等待其他后台流量。
func watchSearchResponses(page *rod.Page, keyword string) (<-chan searchResponse, func()) {
	p, cancel := page.WithCancel()
	responses := make(chan searchResponse, 1)
	requests := make(map[proto.NetworkRequestID]int)
	update := func(result searchResponse) {
		select {
		case <-responses:
		default:
		}
		responses <- result
	}
	wait := p.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
		if !isSearchNotesURL(e.Request.URL) || e.Request.Method != "POST" {
			return
		}
		body := e.Request.PostData
		if body == "" && e.Request.HasPostData {
			data, err := (proto.NetworkGetRequestPostData{RequestID: e.RequestID}).Call(p)
			if err != nil {
				return
			}
			body = data.PostData
		}
		var request struct {
			Keyword string `json:"keyword"`
			Page    int    `json:"page"`
		}
		if json.Unmarshal([]byte(body), &request) == nil && request.Keyword == keyword && request.Page == 1 {
			// 同关键词也可能重新发起搜索，忽略旧请求后到的完成和失败事件。
			clear(requests)
			requests[e.RequestID] = 0
			// 页面可能重试，旧的失败或空响应不能作为新请求的完成信号。
			update(searchResponse{})
		}
	}, func(e *proto.NetworkResponseReceived) {
		if _, ok := requests[e.RequestID]; ok {
			requests[e.RequestID] = e.Response.Status
		}
	}, func(e *proto.NetworkLoadingFinished) {
		status, ok := requests[e.RequestID]
		if !ok {
			return
		}
		delete(requests, e.RequestID)
		if status < 200 || status >= 300 {
			update(searchResponse{err: fmt.Errorf("搜索请求失败（HTTP %d）", status)})
			return
		}
		response, err := (proto.NetworkGetResponseBody{RequestID: e.RequestID}).Call(p)
		if err != nil {
			update(searchResponse{err: fmt.Errorf("读取搜索响应失败: %w", err)})
			return
		}
		body := []byte(response.Body)
		if response.Base64Encoded {
			body, err = base64.StdEncoding.DecodeString(response.Body)
			if err != nil {
				update(searchResponse{err: fmt.Errorf("解码搜索响应失败: %w", err)})
				return
			}
		}
		update(parseSearchResponse(body))
	}, func(e *proto.NetworkLoadingFailed) {
		if _, ok := requests[e.RequestID]; ok {
			delete(requests, e.RequestID)
			update(searchResponse{err: fmt.Errorf("搜索请求未完成: %s", e.ErrorText)})
		}
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		wait()
	}()
	return responses, func() { cancel(); <-done }
}

func isSearchNotesURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || !strings.HasSuffix(u.Hostname(), ".xiaohongshu.com") {
		return false
	}
	return u.Path == "/api/sns/web/v1/search/notes" || u.Path == "/api/sns/web/v2/search/notes"
}

func parseSearchResponse(body []byte) searchResponse {
	var response struct {
		Code    *int `json:"code"`
		Success bool `json:"success"`
		Data    *struct {
			Items   []json.RawMessage `json:"items"`
			HasMore *bool             `json:"has_more"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return searchResponse{err: fmt.Errorf("解析搜索响应失败: %w", err)}
	}
	if response.Code == nil || !response.Success || *response.Code != 0 {
		if response.Code != nil {
			return searchResponse{err: fmt.Errorf("搜索接口返回失败（code=%d）", *response.Code)}
		}
		return searchResponse{err: fmt.Errorf("搜索接口响应缺少成功标记")}
	}
	if response.Data == nil {
		return searchResponse{err: fmt.Errorf("搜索响应缺少 data")}
	}
	if len(response.Data.Items) > 0 {
		return searchResponse{}
	}
	// 实测零结果响应会省略 items，只返回 has_more:false。
	if response.Data.HasMore == nil || *response.Data.HasMore {
		return searchResponse{err: fmt.Errorf("搜索响应未确认结果为空")}
	}
	return searchResponse{empty: true}
}

// searchReadinessJS 只读当前搜索状态；全局对象存在和空态文案都不是完成信号。
//
// 顺带返回当前路径：风控会把主框架延迟重定向到验证页/登录页，那里永远等不到
// 搜索结果，必须让等待方据此提前退出。只取 location.pathname，不含查询串——
// 登录页的 redirectPath 参数里可能带着验证页地址，不能据此误判。
const searchReadinessJS = `(keyword) => {
	const path = location.pathname;
	const unwrap = value => value?.value !== undefined ? value.value :
		(value?._value !== undefined ? value._value : value);
	const search = window.__INITIAL_STATE__?.search;
	const currentKeyword = unwrap(search?.searchContext)?.keyword;
	const urlKeyword = new URL(location.href).searchParams.get('keyword');
	if (currentKeyword !== undefined ? currentKeyword !== keyword : urlKeyword !== keyword) return {path};
	const feeds = unwrap(search?.feeds);
	const state = unwrap(search?.state);
	return {
		path,
		ready: Array.isArray(feeds) && feeds.length > 0 && state !== 'loading' && state !== 'error',
		empty: Array.isArray(feeds) && feeds.length === 0 && state !== 'loading',
		failed: state === 'error'
	};
}`

func waitSearchReady(page *rod.Page, keyword string, responses <-chan searchResponse) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var response searchResponse
	for {
		state, err := page.Eval(searchReadinessJS, keyword)
		if err != nil {
			// 跳转会让执行上下文短暂失效。先确认主框架是否已经落到验证页/登录页——
			// 跳转本身就会让 eval 报错，而拦截页才是真正的原因。
			// 注意：page 的 deadline 已经耗尽时，这里的子超时同样失效、直接返回 nil，
			// 那种情况下面会把 deadline 错误原样返回给调用方。
			if blocked := CheckPageBlocked(page); blocked != nil {
				return blocked
			}
			// 只是跳转期间的上下文失效就重试，别把瞬时错误当成等待结果。
			if isTransientContextError(err) {
				select {
				case <-page.GetContext().Done():
					return page.GetContext().Err()
				case <-ticker.C:
				}
				continue
			}
			return err
		}
		// 主框架进入验证页或登录页时，搜索结果永远不会就绪，立即退出。
		// 站点是延迟重定向：导航返回时页面还是搜索页，所以不能只在导航后判一次。
		// 注意 gson 在 path 缺失时给出 "<nil>"，所以判定必须依赖精确/前缀匹配，
		// 不能改成子串匹配。
		if blocked := classifyBlockedPath(state.Value.Get("path").Str()); blocked != nil {
			return blocked
		}
		// 读取页面期间可能有新请求或响应到达，先消费通知并重读，避免沿用旧快照。
		select {
		case response = <-responses:
			continue
		default:
		}
		if state.Value.Get("ready").Bool() || (state.Value.Get("empty").Bool() && response.empty) {
			return nil
		}
		// 空结果和接口异常都会被站点标为 error，必须结合对应请求的响应区分。
		if state.Value.Get("failed").Bool() && response.err != nil {
			return response.err
		}
		select {
		case <-page.GetContext().Done():
			return page.GetContext().Err()
		case response = <-responses:
		case <-ticker.C:
		}
	}
}

// isTransientContextError 判断 eval 失败是否只是跳转期间执行上下文短暂失效。
// 这类错误应当重试，而不是当成等待结果返回给调用方。
func isTransientContextError(err error) bool {
	return errors.Is(err, cdp.ErrCtxNotFound) ||
		errors.Is(err, cdp.ErrCtxDestroyed) ||
		errors.Is(err, cdp.ErrObjNotFound)
}
