# RedNote fork validation

This fork defaults to `www.rednote.com` and `creator.rednote.com`.
Set `XHS_SITE=xiaohongshu` explicitly for domestic accounts, using the same
setting for `go run ./cmd/login` and the MCP server. Unknown values fail at startup.
Docker Compose forwards `XHS_SITE` with a RedNote default. Use an image built
from this fork; setting the variable does not add support to older upstream
images.

Existing cookies must belong to the selected site; switching the environment
variable does not migrate an account or its session.

The creator session check requires the selected HTTPS origin, the publishing
route, and a file upload control. A cookie alone does not prove that Creator
Center SSO completed. Publish confirmation requires `/publish/success` on the
selected origin; a redirect to login is an error.

`save_draft` supports image posts and the visible buttons `保存草稿`, `暂存离开`,
and `Save draft`. It never selects a publish button. It reports success only
when a visible positive save message appears; an unrecognized confirmation
returns an error rather than claiming success. Web drafts may not sync to the
mobile app. Long-form article creation is not implemented by this MCP tool.

Pass topic names through the `tags` array. Plain `#name` text in `content` is
not converted into a topic automatically. The topic picker must provide an
exact name match, and the editor must gain a matching structured topic node
after selection. A missing candidate or unconfirmed insertion stops the
operation before publishing; it no longer silently falls back to plain text.
Confirmation currently requires an explicit `data-type="topic"` marker. Ordinary
links, CSS classes, and generic read-only nodes are rejected. Editors with another
markup format fail closed; these selectors still require live-editor verification.

## Automated checks

```sh
go test ./...
go vet ./...
go build ./...
go test -tags integration -run '^$' ./...
go test -tags integration ./xiaohongshu \
  -run 'Test(RedNoteLoginStateFixtures|DraftButtonAndConfirmationFixtures|CreatorSessionAndPublishResultFixtures|TopicNodeFixtures|TopicSelectionFixtures)$' \
  -count=1 -timeout=90s
```

The last command runs isolated browser fixtures using local HTML and intercepted
responses. It does not sign in to a real account, save a real draft, or publish.
The other integration tests may contact the live service; do not run the entire
integration suite as a release check.

## Remaining live verification

A direct browser post on RedNote succeeded on 2026-10-24. That verifies the web
workflow, not authenticated publishing through MCP. Before describing the fork
as fully verified, run a fresh login with the selected site, restart the server
with its saved session, and call `save_draft` with a clearly identified test
image. Confirm the draft exists in Creator Center and that no post was published.
A real publish smoke test needs its own explicitly authorized content.

Do not commit cookies, QR codes, account screenshots, or article assets as fixtures.
