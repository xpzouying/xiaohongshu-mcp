package main

import "testing"

func TestInitMCPServerDoesNotPanic(t *testing.T) {
	appServer := &AppServer{xiaohongshuService: NewXiaohongshuService()}
	if server := InitMCPServer(appServer); server == nil {
		t.Fatal("expected MCP server")
	}
}
