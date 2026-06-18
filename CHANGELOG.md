# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased] - 2026-02-16

### Fixed

#### 1) `reply_comment_in_feed` frequently failed with "未找到评论"

- **Root cause**:
  - Comment lookup relied on a narrow set of selectors (legacy DOM assumptions).
  - Loop could terminate too early when `THE END` marker appeared, even before robust lookup had a chance.
- **Fixes** (`xiaohongshu/comment_feed.go`):
  - Lookup order changed: try `comment_id` / `user_id` matching first, then evaluate end-of-list signals.
  - Added more selector variants for comment matching:
    - `#comment-...`, `id*`, `data-comment-id`, `data-commentid`, `data-rid`
  - Added XPath fallback for `user_id` profile-link ancestry matching.
  - Added robust comment counting across multiple selector families.
  - Refined stop logic to avoid premature break on transient bottom markers.
  - Added reply button fallback strategy (class selector + text fallback for "回复").
  - Reduced unproductive long waits and improved diagnostic logs.

#### 2) Login success but daemon still reported "未登录" (cookie path drift)

- **Root cause**:
  - Cookie path default depended on current working directory (`cookies.json`),
    so login binary and daemon could read/write different files.
- **Fixes** (`cookies/cookies.go`):
  - `COOKIES_PATH` now has highest priority.
  - Backward-compatible fallback chain:
    1. `COOKIES_PATH`
    2. legacy `/tmp/cookies.json`
    3. local `./cookies.json` (if exists)
    4. stable default `~/.xiaohongshu-mcp/cookies.json`
  - `SaveCookies()` now auto-creates parent directory when needed.

### Validation

- `go build ./...` passed.
- End-to-end validation on target note:
  - `get_feed_detail` returns target `comment_id` and `user_id`.
  - `reply_comment_in_feed` returns success for existing top-level comment.

### Notes

- This patch focuses on reliability of comment reply discovery and cookie-path stability.
- No intentional breaking API changes.
