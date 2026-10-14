package errors

import "errors"

var ErrNoFeeds = errors.New("没有捕获到 feeds 数据")
var ErrNoFeedDetail = errors.New("没有捕获到 feed 详情数据")

// ErrVerificationRequired 会话被风控判定可疑，请求被重定向到安全验证页。
// 此时页面上不会出现 __INITIAL_STATE__，各流程若只是等超时，只能报出一句
// "context deadline exceeded"，看不出真正原因。
var ErrVerificationRequired = errors.New("小红书要求安全验证：当前会话被判定可疑，已重定向到安全验证页。" +
	"请用手机小红书 App 扫描该页面上的二维码完成身份验证后重试" +
	"（在浏览器打开 https://www.xiaohongshu.com 即可触发并看到该验证页）")

// ErrLoginRequired 未登录或登录态已失效，请求被重定向到登录页。
var ErrLoginRequired = errors.New("未登录或登录态已失效：请求被重定向到登录页，请重新扫码登录")
