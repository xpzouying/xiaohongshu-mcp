package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/xpzouying/headless_browser"
	"github.com/xpzouying/xiaohongshu-mcp/account"
	"github.com/xpzouying/xiaohongshu-mcp/xiaohongshu"
)

type browserAccountLogin struct {
	cookies account.CookieStore
	create  func(string) account.Browser
}

func newBrowserAccountLogin(cookies account.CookieStore) AccountLogin {
	return &browserAccountLogin{cookies: cookies, create: newBrowserWithAccountCookie}
}

func (l *browserAccountLogin) open(accountID string) (*headless_browser.Browser, error) {
	path, err := l.cookies.Path(accountID)
	if err != nil {
		return nil, err
	}
	browser, ok := l.create(path).(*headless_browser.Browser)
	if !ok {
		return nil, &account.Error{Code: account.CodeInternalError, Message: "账号浏览器类型无效"}
	}
	return browser, nil
}

func (l *browserAccountLogin) Status(ctx context.Context, accountID string) (bool, string, error) {
	browser, err := l.open(accountID)
	if err != nil {
		return false, "", err
	}
	defer browser.Close()
	page := browser.NewPage()
	defer page.Close()
	loggedIn, err := xiaohongshu.NewLogin(page).CheckLoginStatus(ctx)
	return loggedIn, "", err
}

func (l *browserAccountLogin) QRCode(ctx context.Context, accountID string) (string, bool, error) {
	browser, err := l.open(accountID)
	if err != nil {
		return "", false, err
	}
	page := browser.NewPage()
	login := xiaohongshu.NewLogin(page)
	image, loggedIn, err := login.FetchQrcodeImage(ctx)
	if err != nil || loggedIn {
		_ = page.Close()
		browser.Close()
		return image, loggedIn, err
	}
	go func() {
		defer page.Close()
		defer browser.Close()
		waitCtx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		if !login.WaitForLogin(waitCtx) {
			return
		}
		cookies, cookieErr := page.Browser().GetCookies()
		if cookieErr != nil {
			return
		}
		data, marshalErr := json.Marshal(cookies)
		if marshalErr != nil {
			return
		}
		_ = l.cookies.Save(context.Background(), accountID, data)
	}()
	return image, false, nil
}
