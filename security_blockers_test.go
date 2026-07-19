package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/xpzouying/xiaohongshu-mcp/account"
)

func TestComposeAuthenticationModesRenderIndependently(t *testing.T) {
	root := t.TempDir()
	secret := func(name string) string {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte(name+"-token"), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	current := []string{
		"XHS_READ_TOKEN_FILE_HOST=" + secret("read"),
		"XHS_WRITE_TOKEN_FILE_HOST=" + secret("write"),
		"XHS_ADMIN_TOKEN_FILE_HOST=" + secret("admin"),
	}
	previous := []string{
		"XHS_READ_TOKEN_PREVIOUS_FILE_HOST=" + secret("read-previous"),
		"XHS_WRITE_TOKEN_PREVIOUS_FILE_HOST=" + secret("write-previous"),
		"XHS_ADMIN_TOKEN_PREVIOUS_FILE_HOST=" + secret("admin-previous"),
	}
	legacy := []string{
		"XHS_API_TOKEN_FILE_HOST=" + secret("legacy"),
		"WEBUI_PASSWORD_FILE_HOST=" + secret("webui-password"),
	}
	unset := []string{
		"XHS_READ_TOKEN_FILE_HOST", "XHS_WRITE_TOKEN_FILE_HOST", "XHS_ADMIN_TOKEN_FILE_HOST",
		"XHS_READ_TOKEN_PREVIOUS_FILE_HOST", "XHS_WRITE_TOKEN_PREVIOUS_FILE_HOST", "XHS_ADMIN_TOKEN_PREVIOUS_FILE_HOST",
	}

	run := func(name string, env []string, files ...string) error {
		t.Helper()
		args := []string{"compose"}
		for _, file := range files {
			args = append(args, "-f", file)
		}
		args = append(args, "config")
		cmd := exec.Command("docker", args...)
		cmd.Env = append(cleanEnvironment(os.Environ(), unset), env...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return &composeRenderError{name: name, output: string(output), err: err}
		}
		return nil
	}

	if err := run("current", current, "docker/docker-compose.yml"); err != nil {
		t.Fatal(err)
	}
	if err := run("overlap", append(current, previous...), "docker/docker-compose.yml", "docker/docker-compose.overlap.yml"); err != nil {
		t.Fatal(err)
	}
	if err := run("legacy backend", legacy, "docker/docker-compose.legacy.yml"); err != nil {
		t.Fatal(err)
	}
	if err := run("legacy webui", legacy, "docker-compose.webui.legacy.yml"); err != nil {
		t.Fatal(err)
	}
	if err := run("missing current scope", nil, "docker/docker-compose.yml"); err == nil {
		t.Fatal("current mode without scoped secrets must fail closed")
	}
}

type composeRenderError struct {
	name, output string
	err          error
}

func (e *composeRenderError) Error() string { return e.name + ": " + e.err.Error() + ": " + e.output }

func cleanEnvironment(env, unset []string) []string {
	clean := make([]string, 0, len(env))
	for _, entry := range env {
		remove := false
		for _, name := range unset {
			if strings.HasPrefix(entry, name+"=") {
				remove = true
				break
			}
		}
		if !remove {
			clean = append(clean, entry)
		}
	}
	return clean
}

func TestRESTWriteHandlersExposeUncertainOutcomeWithoutRetry(t *testing.T) {
	operations := []struct {
		name, path, body, code string
		set                    func(*AppServer, *int, error)
	}{
		{"publish", "/api/v1/publish", `{"account_id":"acct","title":"t","content":"sensitive-content","images":["sensitive-path"]}`, "PUBLISH_UNKNOWN", func(s *AppServer, calls *int, err error) {
			s.publishContent = func(context.Context, *PublishRequest) (*PublishResponse, error) { *calls++; return nil, err }
		}},
		{"publish video", "/api/v1/publish_video", `{"account_id":"acct","title":"t","content":"sensitive-content","video":"sensitive-path"}`, "PUBLISH_VIDEO_UNKNOWN", func(s *AppServer, calls *int, err error) {
			s.publishVideo = func(context.Context, *PublishVideoRequest) (*PublishVideoResponse, error) { *calls++; return nil, err }
		}},
		{"comment", "/api/v1/feeds/comment", `{"account_id":"acct","feed_id":"sensitive-feed","xsec_token":"sensitive-xsec","content":"sensitive-content"}`, "POST_COMMENT_UNKNOWN", func(s *AppServer, calls *int, err error) {
			s.postComment = func(context.Context, string, string, string) (*PostCommentResponse, error) { *calls++; return nil, err }
		}},
		{"reply", "/api/v1/feeds/comment/reply", `{"account_id":"acct","feed_id":"sensitive-feed","xsec_token":"sensitive-xsec","comment_id":"comment","content":"sensitive-content"}`, "REPLY_COMMENT_UNKNOWN", func(s *AppServer, calls *int, err error) {
			s.replyComment = func(context.Context, string, string, string, string, string) (*ReplyCommentResponse, error) {
				*calls++
				return nil, err
			}
		}},
	}
	errors := []struct {
		name string
		err  error
		want int
	}{{"canceled", context.Canceled, http.StatusRequestTimeout}, {"deadline", context.DeadlineExceeded, http.StatusGatewayTimeout}}

	for _, operation := range operations {
		for _, failure := range errors {
			t.Run(operation.name+"/"+failure.name, func(t *testing.T) {
				registry := &routingRegistry{resolved: account.ResolvedAccount{Account: account.Account{ID: "acct", Status: account.StatusActive}}}
				locks, _ := account.NewLockManager(1)
				manager := account.NewAccountManager(registry, locks, routingFactory{browser: &routingBrowser{}})
				server := &AppServer{accountManager: manager}
				calls := 0
				operation.set(server, &calls, failure.err)
				var logs bytes.Buffer
				previous := logrus.StandardLogger().Out
				logrus.SetOutput(&logs)
				defer logrus.SetOutput(previous)

				request := httptest.NewRequest(http.MethodPost, operation.path, strings.NewReader(operation.body))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Authorization", "Bearer write-token")
				response := httptest.NewRecorder()
				setupRoutesWithSecurity(server, scopedTestConfig()).ServeHTTP(response, request)

				var payload ErrorResponse
				if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
					t.Fatal(err)
				}
				if response.Code != failure.want || payload.Code != operation.code {
					t.Fatalf("status=%d code=%q body=%s", response.Code, payload.Code, response.Body.String())
				}
				if calls != 1 {
					t.Fatalf("service calls=%d, want 1", calls)
				}
				output := logs.String()
				if strings.Count(output, "outcome=UNKNOWN") != 1 || strings.Count(output, "event=security_audit") != 1 {
					t.Fatalf("audit log=%s", output)
				}
				for _, secret := range []string{"sensitive-content", "sensitive-path", "sensitive-feed", "sensitive-xsec"} {
					if strings.Contains(output, secret) || strings.Contains(response.Body.String(), secret) {
						t.Fatalf("sensitive value leaked: %q", secret)
					}
				}
			})
		}
	}
}
