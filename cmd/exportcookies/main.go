package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires,omitempty"`
	HTTPOnly bool   `json:"httpOnly"`
	Secure   bool   `json:"secure"`
	SameSite string `json:"sameSite,omitempty"`
}

func main() {
	// OpenClaw Chrome cookies 路径
	home := os.Getenv("HOME")
	if home == "" {
		fmt.Fprintln(os.Stderr, "HOME not set")
		os.Exit(1)
	}
	dbPath := filepath.Join(home, ".openclaw", "browser", "openclaw", "Default", "Cookies")

	// 打开 SQLite 数据库
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开数据库失败: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// 查询 cookies，只取 xiaohongshu.com 相关
	rows, err := db.Query(`
		SELECT name, value, host_key, path, expires_utc, is_secure, is_httponly, same_site 
		FROM cookies 
		WHERE host_key LIKE '%xiaohongshu.com%' OR host_key LIKE '%xhscdn.com%'
	`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "查询失败: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var cookies []Cookie
	for rows.Next() {
		var c Cookie
		var hostKey, sameSite sql.NullString
		var expiresUtc int64
		var isSecure, isHttpOnly int64

		if err := rows.Scan(&c.Name, &c.Value, &hostKey, &c.Path, &expiresUtc, &isSecure, &isHttpOnly, &sameSite); err != nil {
			fmt.Fprintf(os.Stderr, "扫描行失败: %v\n", err)
			continue
		}
		c.Domain = hostKey.String
		c.Secure = isSecure == 1
		c.HTTPOnly = isHttpOnly == 1
		c.Expires = expiresUtc / 1000000 - 11644473600 // 转换为秒级 Unix 时间
		if sameSite.Valid {
			switch sameSite.Int64 {
			case 0:
				c.SameSite = ""
			case 1:
				c.SameSite = "Strict"
			case 2:
				c.SameSite = "Lax"
			case 3:
				c.SameSite = "None"
			}
		}
		cookies = append(cookies, c)
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "遍历行错误: %v\n", err)
		os.Exit(1)
	}

	// 输出为 JSON
	out, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "序列化失败: %v\n", err)
		os.Exit(1)
	}

	// 写入 MCP 的 cookies.json（默认当前目录 cookies.json，也可指定路径）
	envPath := os.Getenv("COOKIES_PATH")
	var outPath string
	if envPath != "" {
		outPath = envPath
	} else {
		outPath = "cookies.json"
	}
	if err := os.WriteFile(outPath, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "写入失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✅ 导出 %d 个 xiaohongshu cookies 到 %s\n", len(cookies), outPath)
}
