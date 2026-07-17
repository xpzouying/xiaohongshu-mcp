'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {
  TOOL_NAMES, ToolInputError, buildRequest, callTool, createToolState
} = require('./mcp-contract.js');

const expectedTools = [
  'check_login_status', 'create_account', 'favorite_feed', 'get_feed_detail',
  'get_login_qrcode', 'like_feed', 'list_accounts', 'list_feeds',
  'post_comment_to_feed', 'publish_content', 'publish_with_video', 'remove_account',
  'reply_comment_in_feed', 'reset_login', 'search_feeds', 'set_default_account', 'user_profile'
];

const validInputs = {
  list_accounts: {},
  create_account: {account_id: 'acct_one', display_name: '账号一', owner: '团队', purpose: '运营'},
  remove_account: {account_id: 'acct_one'},
  set_default_account: {account_id: 'acct_one'},
  check_login_status: {account_id: 'acct_one'},
  get_login_qrcode: {account_id: 'acct_one'},
  reset_login: {account_id: 'acct_one'},
  publish_content: {title: '标题', content: '正文', images: ['https://example.test/a.png'], tags: [], products: []},
  list_feeds: {},
  search_feeds: {keyword: '露营', filters: {sort_by: '最新', note_type: '图文', publish_time: '一周内', search_scope: '未看过', location: '同城'}},
  get_feed_detail: {feed_id: 'feed-1', xsec_token: 'token', load_all_comments: true, limit: 20, click_more_replies: true, reply_limit: 10, scroll_speed: 'normal'},
  user_profile: {user_id: 'user-1', xsec_token: 'token'},
  post_comment_to_feed: {feed_id: 'feed-1', xsec_token: 'token', content: '评论'},
  reply_comment_in_feed: {feed_id: 'feed-1', xsec_token: 'token', comment_id: 'comment-1', content: '回复'},
  publish_with_video: {title: '视频', content: '', video: '/srv/video.mp4', tags: [], products: []},
  like_feed: {feed_id: 'feed-1', xsec_token: 'token', unlike: false},
  favorite_feed: {feed_id: 'feed-1', xsec_token: 'token', unfavorite: false}
};

test('精确暴露 17 个 MCP 工具', () => {
  assert.deepEqual([...TOOL_NAMES].sort(), expectedTools);
});

test('17 个工具均由对应页面通过统一调用层触发', () => {
  const pageScripts = ['app.js', 'accounts.js', 'discover.js', 'publish.js', 'detail.js', 'profile.js'];
  const source = pageScripts.map(file => fs.readFileSync(path.join(__dirname, file), 'utf8')).join('\n');
  for (const tool of expectedTools) {
    assert.ok(source.includes(`'${tool}'`), `${tool} 未接入页面调用层`);
  }
  for (const page of ['accounts.html', 'discover.html', 'publish.html', 'detail.html', 'profile.html']) {
    const html = fs.readFileSync(path.join(__dirname, page), 'utf8');
    assert.ok(html.includes('/static/mcp-contract.js'), `${page} 未加载 MCP 契约层`);
  }
});

test('17 个工具均可独立构建请求并透传取消信号', async () => {
  for (const tool of expectedTools) {
    const request = buildRequest(tool, validInputs[tool]);
    assert.match(request.path, /^\/api\/web\//, tool);
    const calls = [];
    const result = await callTool(tool, validInputs[tool], {
      timeoutMs: 0,
      api: async (path, options) => {
        calls.push({path, options});
        assert.ok(options.signal instanceof AbortSignal);
        return tool === 'user_profile' ? {data: {userBasicInfo: {nickname: '用户'}}} : {ok: true};
      }
    });
    assert.equal(calls.length, 1, tool);
    if (tool === 'user_profile') assert.equal(result.userBasicInfo.nickname, '用户');
  }
});

test('调用层把冻结账号传给 API，避免跟随全局账号漂移', async () => {
  let received;
  await callTool('publish_content', validInputs.publish_content, {
    accountId:'acct_one', timeoutMs:0,
    api:async (_path, options) => { received = options.accountId; return {ok:true}; }
  });
  assert.equal(received, 'acct_one');
});

test('账号创建、搜索筛选与详情高级参数精确序列化', () => {
  assert.deepEqual(buildRequest('create_account', validInputs.create_account).options.body, {
    id: 'acct_one', display_name: '账号一', owner: '团队', purpose: '运营'
  });
  assert.deepEqual(buildRequest('search_feeds', validInputs.search_feeds).options.body.filters, validInputs.search_feeds.filters);
  assert.deepEqual(buildRequest('get_feed_detail', validInputs.get_feed_detail).options.body.comment_config, {
    max_comment_items: 20, click_more_replies: true, scroll_speed: 'normal', max_replies_threshold: 10
  });
  const basic = buildRequest('get_feed_detail', {feed_id: 'f', xsec_token: 't'}).options.body;
  assert.equal(Object.hasOwn(basic, 'comment_config'), false);
});

test('搜索省略默认筛选并精确保留非默认值', () => {
  const defaults = buildRequest('search_feeds', {keyword: '露营', filters: {
    sort_by: '综合', note_type: '不限', publish_time: '不限', search_scope: '不限', location: '不限'
  }}).options.body;
  assert.deepEqual(defaults, {keyword: '露营'});

  const partial = buildRequest('search_feeds', {keyword: '露营', filters: {
    sort_by: '综合', note_type: '图文', publish_time: '不限', search_scope: '不限', location: '不限'
  }}).options.body;
  assert.deepEqual(partial, {keyword: '露营', filters: {note_type: '图文'}});

  assert.deepEqual(buildRequest('search_feeds', {keyword: '露营'}).options.body, {keyword: '露营'});
});

test('关键非法输入在发请求前拒绝', () => {
  const cases = [
    ['create_account', {...validInputs.create_account, account_id: 'root'}],
    ['search_feeds', {...validInputs.search_feeds, keyword: ''}],
    ['search_feeds', {...validInputs.search_feeds, filters: {...validInputs.search_feeds.filters, sort_by: '随机'}}],
    ['get_feed_detail', {...validInputs.get_feed_detail, limit: 0}],
    ['reply_comment_in_feed', {...validInputs.reply_comment_in_feed, comment_id: '', user_id: ''}],
    ['publish_content', {...validInputs.publish_content, images: []}],
    ['publish_with_video', {...validInputs.publish_with_video, video: 'relative.mp4'}]
  ];
  for (const [tool, input] of cases) assert.throws(() => buildRequest(tool, input), ToolInputError, tool);
});

test('超时、主动取消与网络失败保持稳定错误', async () => {
  const never = (_path, options) => new Promise((_resolve, reject) => {
    options.signal.addEventListener('abort', () => reject(Object.assign(new Error('aborted'), {name: 'AbortError'})), {once: true});
  });
  await assert.rejects(callTool('list_feeds', {}, {api: never, timeoutMs: 5}), error => error.code === 'REQUEST_TIMEOUT');
  const controller = new AbortController();
  const pending = callTool('list_feeds', {}, {api: never, signal: controller.signal, timeoutMs: 0});
  controller.abort();
  await assert.rejects(pending, error => error.code === 'REQUEST_ABORTED');
  const network = Object.assign(new Error('offline'), {code: 'NETWORK_ERROR', status: 0});
  await assert.rejects(callTool('list_feeds', {}, {api: async () => { throw network; }}), error => error === network);
});

test('概览页加载统一 MCP 调用层', () => {
  const html = fs.readFileSync(path.join(__dirname, 'index.html'), 'utf8');
  const appIndex = html.indexOf('/static/app.js');
  const contractIndex = html.indexOf('/static/mcp-contract.js');
  const dashboardIndex = html.indexOf('/static/dashboard.js');
  assert.ok(appIndex >= 0, '缺少 app.js');
  assert.ok(contractIndex > appIndex, 'mcp-contract.js 必须在 app.js 之后加载');
  assert.ok(dashboardIndex > contractIndex, 'dashboard.js 必须在 mcp-contract.js 之后加载');
});

test('状态层覆盖 loading/success/empty/error 并取消上一请求', async () => {
  let resolveFirst;
  const calls = [];
  const state = createToolState('list_feeds', {timeoutMs: 0});
  const statuses = [];
  state.subscribe(snapshot => statuses.push(snapshot.status));
  const first = state.run({}, {api: (_path, options) => new Promise((resolve, reject) => {
    resolveFirst = resolve;
    options.signal.addEventListener('abort', () => reject(Object.assign(new Error('aborted'), {name: 'AbortError'})), {once: true});
  })});
  const second = state.run({}, {api: async () => ({feeds: []})});
  await assert.rejects(first, error => error.code === 'REQUEST_ABORTED');
  await second;
  assert.equal(state.get().status, 'empty');
  await state.run({}, {api: async () => ({feeds: [{id: '1'}]})});
  assert.equal(state.get().status, 'success');
  await assert.rejects(state.run({}, {api: async () => { throw new Error('server'); }}), /server/);
  assert.equal(state.get().status, 'error');
  assert.ok(statuses.includes('loading'));
  resolveFirst?.({feeds: []});
  calls.push(true);
});
