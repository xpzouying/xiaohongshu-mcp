// Copyright 2026 Marvelooous
// SPDX-License-Identifier: Apache-2.0

package xiaohongshu

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

func TestNavigateToElementWithSlowResources(t *testing.T) {
	bin := os.Getenv("XHS_TEST_BROWSER")
	if bin == "" {
		t.Skip("set XHS_TEST_BROWSER for real-browser navigation regression test")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow-image" {
			<-r.Context().Done()
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><div id="ready">Ready</div><img src="/slow-image"></body></html>`)
	}))
	defer server.Close()
	l := launcher.New().Bin(bin).Headless(true).Leakless(false)
	url, err := l.Launch()
	if err != nil {
		t.Fatal(err)
	}
	defer l.Cleanup()
	browser := rod.New().ControlURL(url)
	if err := browser.Connect(); err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		t.Fatal(err)
	}
	defer page.Close()

	t.Run("does not wait for stalled image", func(t *testing.T) {
		pp := page.Timeout(10 * time.Second)
		if err := navigateToElement(pp, server.URL, "#ready"); err != nil {
			t.Fatal(err)
		}
		state, err := pp.Eval(`() => document.readyState`)
		if err != nil {
			t.Fatal(err)
		}
		if state.Value.Str() == "complete" {
			t.Fatal("fixture should keep the load event pending")
		}
	})
	t.Run("missing UI respects caller deadline", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := navigateToElement(page.Context(ctx), server.URL, "#missing"); err == nil {
			t.Fatal("expected missing selector to return an error")
		}
	})
}
