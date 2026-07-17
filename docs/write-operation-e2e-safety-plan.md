# 写操作 E2E 风险与可逆测试方案

> 范围：`like_feed`、`favorite_feed`、`post_comment_to_feed`、`reply_comment_in_feed`、`publish_content`、`publish_with_video`、`reset_login`、`remove_account`、`set_default_account`、`create_account`。
>
> 本文仅设计和审计，不授权、也不执行任何生产写操作。只读检查不等于写操作授权。

## 1. 结论与安全边界

最小安全策略不是“把所有接口都跑一遍”，而是按副作用分层：

1. **本地契约层**：默认 CI 必跑，不启动真实浏览器，不访问小红书。
2. **隔离无效负载层**：默认 CI 必跑；请求必须在本地校验、临时 registry/cookie store 或断网 browser fake 中失败，断言没有外部调用。
3. **生产可逆操作层**：默认关闭；仅允许点赞/收藏、空壳账号创建、默认账号切换，并要求专用测试账号、逐次授权、前后状态读取和补偿动作。
4. **必须人工验收层**：评论、回复、图文/视频发布、登录重置、账号删除。它们在当前项目能力内无法可靠自动回滚，禁止无人值守 CI 执行。

关键发现：

- 当前 HTTP/MCP 服务没有认证、授权或专用审计存储；`middleware.go` 还允许 `Access-Control-Allow-Origin: *`。在服务暴露给非可信网络前，不应开放任何写 E2E。
- 日志只有 method/path/account/status，缺少 actor、授权票据、目标资源、前后状态、补偿结果和 request ID；不能作为生产写测试的完整审计证据。
- 评论/回复没有删除接口；发布响应没有稳定返回可用于删除的笔记 ID，项目也没有删除笔记接口。因此它们对本系统而言不可逆。
- `remove_account` 对执行过程有 cookie staging/rollback，可防止内部部分失败；但成功完成后没有恢复 API，因此对调用者仍是不可逆操作。
- 点赞/收藏 API 有反向动作且目标态幂等，但当前底层在状态复核失败或两次点击后仍可能返回 nil；不能只凭 HTTP 200 判定成功，必须重新读取详情验证目标态。

## 2. 风险分级与术语

- **R1 低风险**：仅本地状态；可自动清理，无外部可见性。
- **R2 中风险**：外部可见但可通过公开反向动作恢复；恢复后仍可能留下平台风控/审计痕迹。
- **R3 高风险**：外部可见且当前系统不能可靠恢复，或会销毁凭据/账号配置；必须人工验收。
- **可逆**：存在自动化反向 API，并能读取前后状态证明恢复。
- **补偿性可逆**：能恢复业务状态，但不能抹去通知、风控、平台日志等历史痕迹。
- **不可逆**：当前项目没有可靠 API/标识/权限执行恢复；不能把“人工去 App 删除”写成自动回滚。

## 3. 逐操作风险矩阵

| 操作 | 风险/可逆性 | 外部可见性 | 测试账号与目标要求 | 清理/回滚路径 | 用户授权点 |
|---|---|---|---|---|---|
| `like_feed` | R2，补偿性可逆 | 作者可能收到通知；计数和推荐信号可能变化 | 专用 active 测试账号；目标必须是测试团队自有笔记，禁止随机用户笔记 | 先读 `liked=S0`；写目标态 `!S0`；重读验证；调用反向动作恢复 `S0`；再次重读。任一步不确定即停止，不盲目重试 | 每次生产运行前批准账号、feed、时间窗；授权必须明确包含“可能触发通知” |
| `favorite_feed` | R2，补偿性可逆 | 收藏计数/推荐信号可变；通常作者侧可感知 | 同上，自有笔记 | 先读 `collected=S0`；切换并验证；反向恢复；再次验证 | 同上 |
| `post_comment_to_feed` | R3，不可逆（当前无删除接口/返回 comment ID） | 立即公开或按笔记可见范围展示，可能通知作者 | 专用测试账号 + 自有且明确标记为 E2E 的笔记；建议笔记设为仅自己可见 | 当前无自动清理。只能由人工在 App/创作中心删除并截图/记录；删除前不得标记测试通过 | 提交前展示最终文本、账号、笔记、可见范围，要求一次性人工确认；人工删除也是单独授权动作 |
| `reply_comment_in_feed` | R3，不可逆（当前无删除接口） | 目标用户可能收到通知；公开展示上下文 | 专用测试账号；仅回复测试团队自有测试评论；comment_id 优先于 user_id | 当前无自动清理；人工删除回复并验收 | 提交前确认目标 comment/user 与最终文本；明确通知风险 |
| `publish_content` | R3，不可逆（无删除笔记 API，响应契约无可靠 post ID） | 可公开、仅自己或互关可见；可能被推荐、绑定商品或声明原创 | 专用发布测试账号；无真实粉丝/商业绑定；素材已获授权；禁止真实商品绑定；首轮仅自己可见 | 当前无自动回滚。若误发，人工进入创作中心删除；定时发布则在生效前人工取消。必须保存人工清理证据 | 发布前逐项确认标题、正文、图片 hash/路径、标签、可见性、定时、原创、商品；最后点击由人执行 |
| `publish_with_video` | R3，不可逆，且处理时间长 | 同发布；上传和转码也会形成平台侧记录 | 同上；使用短小、无版权争议、无 PII 的测试视频；仅服务器绝对路径 | 当前无自动回滚；人工删除/取消定时。失败后先人工查创作中心，禁止因超时自动重试，避免重复发布 | 同图文发布，并确认视频文件 hash、大小和时长 |
| `reset_login` | R3，破坏性且业务上不可逆 | 不对公众可见，但会使账号离线并需要重新扫码 | 仅可牺牲测试账号；必须有账号持有人在线扫码恢复；禁止生产主账号 | 无安全自动回滚：不应为测试复制/长期保存生产 cookie。清理是重新扫码并确认 `active` | 明确确认 account_id、停机影响、持有人和恢复窗口；执行与重新登录都需人工在场 |
| `remove_account` | R3，成功后不可逆 | 不对平台公众可见；本地 registry、默认选择和 cookie 被删除 | 仅刚创建、未登录、无 cookie 的临时空壳账号；禁止 active/默认/生产账号 | 允许的唯一安全场景：删除同次测试创建的未登录空壳账号。若已有 cookie，当前无恢复 API；内部 transaction rollback 只覆盖执行失败，不覆盖成功后的撤销 | 删除前确认 account_id、状态、是否默认、cookie 是否存在；必须输入完整 account_id 二次确认 |
| `set_default_account` | R1/R2，本地可逆 | 不对小红书公众可见，但会改变后续未显式指定账号的写入目标 | 至少两个专用账号；测试期间所有业务请求仍须显式传 account_id | 记录旧默认 `D0`；设为测试账号；读列表验证；恢复 `D0` 并再验证。若原来无默认，当前没有“清空默认”API，故该场景不可自动恢复，禁止生产 E2E | 确认旧/新默认；批准测试窗口；若无法恢复到“无默认”则不执行 |
| `create_account` | R1（仅空壳），条件可逆 | 不对平台公众可见；写本地 registry | 使用唯一前缀 `e2e_<run>`，`needs_login`，不得扫码，不得设默认 | 创建后读列表验证；调用 `remove_account`；确认 registry 无记录且 cookie 不存在。若中途登录/写入 cookie，立即升级 R3 并停止自动清理 | 创建空壳可按一次测试运行授权；任何扫码登录必须另行人工授权 |

## 4. 最小安全 E2E 分层

### L0：前置静态门禁

所有层都必须先满足：

- 测试清单精确等于上述 10 个写工具，不把只读工具混入写授权。
- MCP annotation 中破坏性工具保持 `DestructiveHint=true`；但 annotation 只用于提示，不能代替服务端授权。
- 测试 runner 默认 `XHS_E2E_WRITE=0`；生产写层还需 `XHS_E2E_PROD_WRITE=1`、短时授权票据和 allowlist 三重开启。
- 服务仅监听 loopback 或受认证的内网反向代理；在没有鉴权时，禁止公网/LAN 暴露写 API。
- 任何日志、报告不得记录 cookie、完整 `xsec_token`、二维码、正文中的 PII 或本地绝对素材路径；token 只记录 hash 前 8 位用于关联。

### L1：本地契约（CI 必跑）

目标：证明输入校验、REST 映射、账号路由、错误信封、并发锁和结果语义，不启动 Rod 浏览器。

建议自动化用例：

1. `webui/static/mcp-contract.test.js`
   - 10 个写工具的 method/path/body 精确断言。
   - 必填字段、枚举、标题/正文长度、schedule 1h～14d、视频绝对路径、回复 `required_without`。
   - 双击/并发只发一次，Abort/timeout 不自动重试写操作。
2. Go handler/MCP 测试
   - fake service 记录调用次数和参数；无合法账号、非 active 状态、busy、取消时业务 handler 调用为 0。
   - 显式 `account_id` 优先，禁止测试依赖默认账号猜测。
   - HTTP 非 2xx、MCP `IsError`、panic recovery 均不产生第二次调用。
3. 账号持久层
   - `create → set default → restore → remove` 使用 `t.TempDir()`。
   - remove 的 cookie stage/commit/registry failure/rollback/crash recovery 全覆盖。
   - reset 只操作临时 cookie store，并断言其他账号不受影响。

本层验收：无网络请求、无真实 cookie、无浏览器进程；测试结束临时目录自动删除。

### L2：隔离无效负载（CI 必跑）

目标：证明恶意/错误输入在产生副作用前失败。

运行环境：临时 registry + fake/断网 browser factory；对 `xiaohongshu.com` 的 DNS/egress 明确拒绝；使用 canary fake，任何 browser 方法调用都使测试失败。

至少覆盖：

- like/favorite/comment/reply：空 feed/token/content、reply 无 comment_id/user_id、超大内容、错误类型。
- publish：无图片、21 字标题、正文越界、非法 visibility、过期/超 14 天 schedule、空数组元素、不可读路径。
- publish video：相对路径、目录、缺失文件、非法扩展/超限文件（待服务端补校验后作为门禁）。
- account：非法/保留/重复 ID、不存在账号、disabled/risk_hold/paused、busy、超 1 MiB body。
- reset/remove/default：不存在账号、busy、无可恢复旧默认。

每例必须断言：返回稳定错误码、外部调用计数为 0、registry/cookie 快照未变、日志无敏感值。

### L3：生产可逆操作（手动触发，默认关闭）

仅允许以下场景：

- `like_feed`：自有测试笔记，状态切换后恢复。
- `favorite_feed`：自有测试笔记，状态切换后恢复。
- `create_account`：创建未登录空壳账号后删除。
- `set_default_account`：仅当存在可记录并恢复的旧默认时切换后恢复。

执行流程必须是固定状态机：

```text
PRECHECK → AUTHORIZED → SNAPSHOT → APPLY → VERIFY → COMPENSATE → VERIFY_RESTORED → CLOSED
                              ↘ UNKNOWN/FAILED → FREEZE → HUMAN_RECONCILE
```

强制规则：

- 每次只执行一个 operation/target；禁止并行。
- 写请求必须带唯一 `run_id`/`request_id`；runner 维护本地 write-ahead journal。
- `APPLY` 超时或连接断开时状态是 `UNKNOWN`，先只读查询真实状态；禁止自动重放。
- 反向动作失败时停止整个 suite，账号进入 `risk_hold` 或 runner 本地 denylist，等待人工处置。
- 补偿成功只代表业务状态恢复，不代表通知/平台审计痕迹被删除。

L3 通过标准：apply 后目标态被只读验证，compensate 后原状态被再次验证，审计事件完整且未出现未授权目标。

### L4：必须人工验收（禁止无人值守）

包含：comment、reply、publish_content、publish_with_video、reset_login、remove_account（除同次创建的未登录空壳清理）。

建议采用“两阶段提交式”操作台：

1. **准备阶段**：系统校验输入、生成 payload 摘要和素材 hash、只读预览、显示风险与清理责任人；不得触发最终提交按钮。
2. **人工提交阶段**：用户在 5 分钟内输入一次性确认短语并点击；确认短语绑定 `operation + account_id + target + payload_hash`，超时失效。
3. **人工验收阶段**：在小红书 App/创作中心核对真实结果；评论/发布记录资源 ID（若可取得）及截图。
4. **人工清理阶段**：由账号持有人删除内容或重新登录；再次核验并关闭工单。

发布类建议首轮使用 `visibility=仅自己可见`，但这仍是生产写操作，并不降级为 L3。

## 5. 可执行测试清单

### 5.1 本地自动化命令

这些命令只运行现有本地测试，不授权生产写入：

```bash
node --test webui/static/*.test.js
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
git diff --check
```

新增 E2E runner 时，默认命令应保持安全失败：

```bash
# L1/L2：允许在 CI 自动执行
XHS_E2E_WRITE=0 go test ./... -run 'Contract|InvalidPayload|Account.*Rollback' -count=1

# L3：示意；缺任一门禁即退出，不得降级执行
XHS_E2E_WRITE=1 \
XHS_E2E_PROD_WRITE=1 \
XHS_E2E_ACCOUNT_ID=e2e_account \
XHS_E2E_ALLOWED_FEED_IDS=owned_test_feed_id \
XHS_E2E_APPROVAL_TOKEN_FILE=/run/secrets/xhs-e2e-approval \
go test ./e2e -run TestReversibleProductionWrites -count=1
```

runner 必须拒绝命令行直接传 token/cookie，避免进入 shell history；授权票据从权限为 `0600` 的短期文件读取，使用后销毁。

### 5.2 每次生产测试 runbook

- [ ] 工单列出 operation、account_id、target、payload hash、执行人、批准人、时间窗和清理责任人。
- [ ] 账号为专用测试账号，状态 active，无 risk hold；业务目标在 allowlist。
- [ ] 只读预检记录原状态；账号/目标与授权票据完全匹配。
- [ ] 确认当前无其他写任务，取得账号锁。
- [ ] 单次执行；保存 HTTP/MCP 结果和 request ID，不保存 secret。
- [ ] 只读验证真实目标态，不以“返回成功”代替验证。
- [ ] L3 立即执行补偿并验证恢复；L4 转人工验收/清理。
- [ ] 若结果未知，禁止重试，先人工对账。
- [ ] 审计记录完成并由第二人复核后关闭。

## 6. 审计日志最低规范

建议输出结构化 JSON Lines 到独立 append-only sink，而不是仅依赖应用日志：

```json
{
  "timestamp": "RFC3339Nano",
  "run_id": "uuid",
  "request_id": "uuid",
  "actor_id": "内部用户/服务身份",
  "approval_id": "短期授权编号",
  "operation": "like_feed",
  "account_id": "e2e_account",
  "target_type": "feed",
  "target_id_hash": "sha256:...",
  "payload_hash": "sha256:...",
  "phase": "precheck|apply|verify|compensate|close",
  "before": {"liked": false},
  "after": {"liked": true},
  "outcome": "success|failed|unknown",
  "error_code": "",
  "duration_ms": 0
}
```

要求：

- actor 与 approval 可追溯；approval 明确绑定操作和目标，不能复用为全局写权限。
- 对 feed/comment/user ID 默认 hash；若排障必须保存明文，应使用受控加密字段和短保留期。
- 永不写 cookie、二维码、完整 xsec_token、素材内容或评论/正文全文。
- journal 在 apply 前落盘；每个 phase 都追加事件，不覆盖历史。
- 保留期、访问控制和删除策略由用户决定；至少满足事故调查窗口。

## 7. 上线前阻断项

在启用任何 L3/L4 生产写测试前，至少补齐：

1. 服务端认证与细粒度授权（按 operation/account/target）；Web UI 的二次确认不能代替 API 授权。
2. 将 CORS 从 `*` 收紧到可信 origin，并配置 CSRF/同源策略或使用不可被浏览器跨站调用的认证方案。
3. request ID、结构化审计 journal、敏感字段脱敏和 approval 校验。
4. 写操作幂等/对账策略：特别是发布、评论和回复在超时后的 `UNKNOWN` 处理；没有 idempotency key 时禁止自动重试。
5. 点赞/收藏：状态验证失败或最终态不匹配必须返回错误，不能返回成功。
6. 评论/回复：若要自动化清理，需返回稳定 comment/reply ID 并实现删除接口；否则永久留在 L4。
7. 发布：需返回稳定 post ID、提供删除/取消定时发布能力并验证最终状态；否则永久留在 L4。
8. reset/remove：增加显式备份/恢复产品设计前，不得宣称可逆；cookie 备份必须单独威胁建模，不应作为测试捷径。

## 8. 需要用户决策的清单

1. **是否允许任何生产写 E2E？** 默认建议只批准 L1/L2，L3/L4 保持关闭。
2. **测试账号**：指定 owner、用途、是否有真实粉丝/商业权限、允许的停机窗口；禁止复用主运营账号。
3. **测试目标**：提供团队自有 feed allowlist；是否接受点赞/收藏可能产生通知和推荐信号。
4. **内容可见性**：评论/回复无法设为仅自己可见，是否接受其公开性；发布首轮是否强制仅自己可见。
5. **人工清理责任**：指定谁负责删除评论/回复/笔记、重新扫码，及完成时限和证据形式。
6. **授权模型**：批准双人复核还是单人确认；approval 有效期、一次性范围和紧急停止人。
7. **审计策略**：日志保存位置、保留期、谁可访问、资源 ID 明文还是 hash。
8. **失败策略**：补偿失败是否自动把账号置为 `risk_hold`，以及谁有权解除。
9. **发布/商品/原创**：是否永久禁止 E2E 绑定商品、声明原创和公开发布（建议禁止）。
10. **账号删除范围**：是否仅允许删除同一 run 创建且从未登录的空壳账号（建议是）。

## 9. 与当前实现的对应证据

- 点赞、收藏、评论、回复和发布等业务写 REST 路由使用 `account.OperationWrite`：`routes.go`；账号管理写路由则直接进入 `AccountTools` 并使用 registry/management 锁与持久层约束。
- 账号重置会删除 cookie 并将状态更新为 `needs_login`：`account_tools.go`。
- 账号删除使用 cookie staging、commit、registry remove 与 rollback：`account/management.go`；这只保证事务失败恢复，不提供成功后的用户撤销。
- 点赞/收藏有目标态幂等和反向动作，但最终复核失败仍可能返回 nil：`xiaohongshu/like_favorite.go`。
- 评论/回复仅有创建动作：`xiaohongshu/comment_feed.go`、`routes.go`。
- 图文/视频发布最终会点击发布按钮，当前无删除路由：`xiaohongshu/publish.go`、`routes.go`。
- 当前日志不具备完整审计字段且 CORS 为 `*`：`handlers_api.go`、`middleware.go`。
