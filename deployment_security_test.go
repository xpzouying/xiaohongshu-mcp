package main

import (
	"os"
	"strings"
	"testing"
)

func TestWebUIImageUserCanReadOwnerOnlyLocalComposeSecrets(t *testing.T) {
	dockerfile, err := os.ReadFile("webui/Dockerfile")
	if err != nil {
		t.Fatal(err)
	}
	content := string(dockerfile)
	for _, required := range []string{"addgroup -S -g 1000 webui", "adduser -S -D -H -u 1000 -G webui webui", "USER webui"} {
		if !strings.Contains(content, required) {
			t.Fatalf("webui/Dockerfile 缺少安全运行用户配置 %q", required)
		}
	}
}

func TestBackendComposePassesExactCORSAllowlistConfiguration(t *testing.T) {
	compose, err := os.ReadFile("docker/docker-compose.yml")
	if err != nil {
		t.Fatal(err)
	}
	const required = "XHS_CORS_ALLOWED_ORIGINS=${XHS_CORS_ALLOWED_ORIGINS:-}"
	if !strings.Contains(string(compose), required) {
		t.Fatalf("docker/docker-compose.yml 缺少 %q", required)
	}
}

func TestCurrentComposeDoesNotInjectEmptyOptionalTokenPaths(t *testing.T) {
	for _, path := range []string{"docker/docker-compose.yml", "docker-compose.webui.yml"} {
		compose, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		content := string(compose)
		for _, name := range []string{
			"XHS_API_TOKEN_FILE",
			"XHS_READ_TOKEN_PREVIOUS_FILE",
			"XHS_WRITE_TOKEN_PREVIOUS_FILE",
			"XHS_ADMIN_TOKEN_PREVIOUS_FILE",
		} {
			if strings.Contains(content, name+"=${"+name+":-}") || strings.Contains(content, name+`: "${`+name+`:-}"`) {
				t.Fatalf("%s 不应注入空的可选路径 %s", path, name)
			}
		}
	}
}
