package xiaohongshu

// Close 关闭发布页面，避免打开多个创作页导致资源泄漏。
func (p *PublishAction) Close() error {
	if p == nil || p.page == nil {
		return nil
	}
	return p.page.Close()
}
