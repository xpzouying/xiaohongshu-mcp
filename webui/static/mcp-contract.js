(function (root, factory) {
  const contract = factory();
  root.XHSMCP = contract;
  if (typeof module === 'object' && module.exports) module.exports = contract;
})(typeof globalThis !== 'undefined' ? globalThis : window, function () {
  'use strict';

  const ACCOUNT_ID_PATTERN = /^[a-z][a-z0-9_]{2,31}$/;
  const RESERVED_ACCOUNT_IDS = new Set(['accounts', 'system', 'root', 'null', 'unknown']);
  const VISIBILITIES = new Set(['公开可见', '仅自己可见', '仅互关好友可见']);
  const SCROLL_SPEEDS = new Set(['slow', 'normal', 'fast']);
  const FILTERS = {
    sort_by: new Set(['综合', '最新', '最多点赞', '最多评论', '最多收藏']),
    note_type: new Set(['不限', '视频', '图文']),
    publish_time: new Set(['不限', '一天内', '一周内', '半年内']),
    search_scope: new Set(['不限', '已看过', '未看过', '已关注']),
    location: new Set(['不限', '同城', '附近'])
  };
  const DEFAULT_FILTERS = {
    sort_by: '综合',
    note_type: '不限',
    publish_time: '不限',
    search_scope: '不限',
    location: '不限'
  };

  class ToolInputError extends Error {
    constructor(message, details) {
      super(message);
      this.name = 'ToolInputError';
      this.code = 'INVALID_TOOL_INPUT';
      this.status = 0;
      this.details = details;
    }
  }

  function requiredString(input, key) {
    const value = String(input[key] ?? '').trim();
    if (!value) throw new ToolInputError(`${key} 不能为空`, {field: key});
    return value;
  }

  function optionalString(input, key) {
    const value = input[key];
    return value == null ? '' : String(value).trim();
  }

  function accountId(input) {
    const value = requiredString(input, 'account_id');
    if (!ACCOUNT_ID_PATTERN.test(value) || RESERVED_ACCOUNT_IDS.has(value)) {
      throw new ToolInputError('account_id 格式无效', {field: 'account_id'});
    }
    return value;
  }

  function stringArray(input, key, required = false) {
    const value = input[key] == null ? [] : input[key];
    if (!Array.isArray(value) || value.some(item => typeof item !== 'string' || !item.trim())) {
      throw new ToolInputError(`${key} 必须是非空字符串数组`, {field: key});
    }
    if (required && value.length === 0) throw new ToolInputError(`${key} 至少需要一项`, {field: key});
    return value.map(item => item.trim());
  }

  function positiveInteger(input, key, fallback) {
    const value = input[key] == null || input[key] === '' ? fallback : Number(input[key]);
    if (!Number.isInteger(value) || value <= 0) throw new ToolInputError(`${key} 必须是正整数`, {field: key});
    return value;
  }

  function schedule(value, now = Date.now()) {
    if (!value) return '';
    const timestamp = Date.parse(value);
    if (!Number.isFinite(timestamp) || timestamp < now + 60 * 60 * 1000 || timestamp > now + 14 * 24 * 60 * 60 * 1000) {
      throw new ToolInputError('schedule_at 须为 1 小时至 14 天内的 ISO8601 时间', {field: 'schedule_at'});
    }
    return new Date(timestamp).toISOString();
  }

  function title(input) {
    const value = requiredString(input, 'title');
    if ([...value].length > 20) throw new ToolInputError('title 不能超过 20 个字符', {field: 'title'});
    return value;
  }

  function publishingBody(input, video) {
    const content = String(input.content ?? '');
    if ([...content].length > 1000) throw new ToolInputError('content 不能超过 1000 个字符', {field: 'content'});
    const visibility = input.visibility || '公开可见';
    if (!VISIBILITIES.has(visibility)) throw new ToolInputError('visibility 无效', {field: 'visibility'});
    const body = {
      title: title(input), content,
      tags: stringArray(input, 'tags'), products: stringArray(input, 'products'),
      visibility, schedule_at: schedule(input.schedule_at)
    };
    if (video) {
      body.video = requiredString(input, 'video');
      if (!(body.video.startsWith('/') || /^[a-z]:[\\/]/i.test(body.video) || body.video.startsWith('\\\\'))) {
        throw new ToolInputError('video 必须是服务端本地绝对路径', {field: 'video'});
      }
    } else {
      body.images = stringArray(input, 'images', true);
      body.is_original = Boolean(input.is_original);
    }
    return body;
  }

  function feedTarget(input) {
    return {feed_id: requiredString(input, 'feed_id'), xsec_token: requiredString(input, 'xsec_token')};
  }

  function actionRequest(path, input, flag) {
    return {path, options: {method: 'POST', body: {...feedTarget(input), [flag]: Boolean(input[flag])}}};
  }

  const builders = {
    list_accounts: () => ({path: '/api/web/accounts', options: {account: false}}),
    create_account: input => ({path: '/api/web/accounts', options: {method: 'POST', account: false, body: {
      id: accountId(input), display_name: requiredString(input, 'display_name'),
      owner: optionalString(input, 'owner'), purpose: optionalString(input, 'purpose')
    }}}),
    remove_account: input => ({path: `/api/web/accounts/${encodeURIComponent(accountId(input))}`, options: {method: 'DELETE', account: false}}),
    set_default_account: input => ({path: `/api/web/accounts/${encodeURIComponent(accountId(input))}/default`, options: {method: 'PUT', account: false}}),
    check_login_status: input => ({path: `/api/web/accounts/${encodeURIComponent(accountId(input))}/login/status`, options: {method: 'POST', body: {}, account: false}}),
    get_login_qrcode: input => ({path: `/api/web/accounts/${encodeURIComponent(accountId(input))}/login/qrcode`, options: {method: 'POST', body: {}, account: false}}),
    reset_login: input => ({path: `/api/web/accounts/${encodeURIComponent(accountId(input))}/login`, options: {method: 'DELETE', account: false}}),
    publish_content: input => ({path: '/api/web/publish', options: {method: 'POST', body: publishingBody(input, false)}}),
    list_feeds: () => ({path: '/api/web/feeds/list', options: {method: 'GET'}}),
    search_feeds: input => {
      const filters = {};
      Object.entries(FILTERS).forEach(([key, allowed]) => {
        const value = input.filters?.[key] ?? DEFAULT_FILTERS[key];
        if (!allowed.has(value)) throw new ToolInputError(`${key} 无效`, {field: `filters.${key}`});
        if (value !== DEFAULT_FILTERS[key]) filters[key] = value;
      });
      const body = {keyword: requiredString(input, 'keyword')};
      if (Object.keys(filters).length > 0) body.filters = filters;
      return {path: '/api/web/feeds/search', options: {method: 'POST', body}};
    },
    get_feed_detail: input => {
      const body = {...feedTarget(input), load_all_comments: Boolean(input.load_all_comments)};
      if (body.load_all_comments) {
        const speed = input.scroll_speed || 'normal';
        if (!SCROLL_SPEEDS.has(speed)) throw new ToolInputError('scroll_speed 无效', {field: 'scroll_speed'});
        body.comment_config = {
          max_comment_items: positiveInteger(input, 'limit', 20),
          click_more_replies: Boolean(input.click_more_replies),
          scroll_speed: speed
        };
        if (body.comment_config.click_more_replies) body.comment_config.max_replies_threshold = positiveInteger(input, 'reply_limit', 10);
      }
      return {path: '/api/web/feeds/detail', options: {method: 'POST', body}};
    },
    user_profile: input => ({path: '/api/web/user/profile', options: {method: 'POST', body: {user_id: requiredString(input, 'user_id'), xsec_token: requiredString(input, 'xsec_token')}}}),
    post_comment_to_feed: input => ({path: '/api/web/feeds/comment', options: {method: 'POST', body: {...feedTarget(input), content: requiredString(input, 'content')}}}),
    reply_comment_in_feed: input => {
      const body = {...feedTarget(input), content: requiredString(input, 'content'), comment_id: optionalString(input, 'comment_id'), user_id: optionalString(input, 'user_id')};
      if (!body.comment_id && !body.user_id) throw new ToolInputError('comment_id 与 user_id 至少提供一个', {fields: ['comment_id', 'user_id']});
      return {path: '/api/web/feeds/comment/reply', options: {method: 'POST', body}};
    },
    publish_with_video: input => ({path: '/api/web/publish_video', options: {method: 'POST', body: publishingBody(input, true)}}),
    like_feed: input => actionRequest('/api/web/feeds/like', input, 'unlike'),
    favorite_feed: input => actionRequest('/api/web/feeds/favorite', input, 'unfavorite')
  };

  const TOOL_NAMES = Object.freeze(Object.keys(builders));

  function buildRequest(toolName, input = {}) {
    const builder = builders[toolName];
    if (!builder) throw new ToolInputError(`未知 MCP 工具: ${toolName}`, {tool: toolName});
    return builder(input);
  }

  function validateToolInput(toolName, input = {}) {
    buildRequest(toolName, input);
    return true;
  }

  function serializeToolInput(toolName, input = {}) {
    return buildRequest(toolName, input).options.body;
  }

  function normalizeResult(toolName, data) {
    if (toolName === 'user_profile' && data && Object.prototype.hasOwnProperty.call(data, 'data')) return data.data;
    return data;
  }

  function isEmptyResult(toolName, data) {
    if (Array.isArray(data)) return data.length === 0;
    if (toolName === 'list_accounts') return Array.isArray(data?.accounts) && data.accounts.length === 0;
    if (toolName === 'list_feeds' || toolName === 'search_feeds') return Array.isArray(data?.feeds) && data.feeds.length === 0;
    return false;
  }

  async function callTool(toolName, input = {}, options = {}) {
    const api = options.api || (typeof globalThis !== 'undefined' && globalThis.XHS?.api);
    if (typeof api !== 'function') throw new Error('XHS.api 尚未初始化');
    const request = buildRequest(toolName, input);
    const timeoutMs = options.timeoutMs ?? 30000;
    const controller = new AbortController();
    let timedOut = false;
    const abort = () => controller.abort(options.signal?.reason);
    if (options.signal?.aborted) abort();
    else options.signal?.addEventListener('abort', abort, {once: true});
    const timer = timeoutMs > 0 ? setTimeout(() => { timedOut = true; controller.abort(); }, timeoutMs) : null;
    try {
      const data = await api(request.path, {...request.options, signal: controller.signal});
      return normalizeResult(toolName, data);
    } catch (error) {
      if (timedOut) {
        const timeoutError = new Error(`请求超时 (${timeoutMs}ms)`);
        timeoutError.name = 'TimeoutError'; timeoutError.code = 'REQUEST_TIMEOUT'; timeoutError.status = 0;
        throw timeoutError;
      }
      if (controller.signal.aborted || error?.name === 'AbortError') {
        const abortError = new Error('请求已取消');
        abortError.name = 'AbortError'; abortError.code = 'REQUEST_ABORTED'; abortError.status = 0;
        throw abortError;
      }
      throw error;
    } finally {
      if (timer) clearTimeout(timer);
      options.signal?.removeEventListener('abort', abort);
    }
  }

  function createToolState(toolName, defaults = {}) {
    let snapshot = Object.freeze({status: 'idle', data: null, error: null});
    let activeController = null;
    const listeners = new Set();
    const publish = next => { snapshot = Object.freeze(next); listeners.forEach(listener => listener(snapshot)); };
    return {
      get: () => snapshot,
      subscribe(listener) { listeners.add(listener); listener(snapshot); return () => listeners.delete(listener); },
      cancel() { activeController?.abort(); },
      reset() { activeController?.abort(); publish({status: 'idle', data: null, error: null}); },
      async run(input = {}, options = {}) {
        activeController?.abort();
        const controller = new AbortController(); activeController = controller;
        publish({status: 'loading', data: snapshot.data, error: null});
        try {
          const data = await callTool(toolName, input, {...defaults, ...options, signal: controller.signal});
          const status = isEmptyResult(toolName, data) ? 'empty' : 'success';
          if (activeController === controller) publish({status, data, error: null});
          return data;
        } catch (error) {
          if (activeController === controller) publish({status: error.code === 'REQUEST_ABORTED' ? 'idle' : 'error', data: snapshot.data, error});
          throw error;
        } finally {
          if (activeController === controller) activeController = null;
        }
      }
    };
  }

  return {TOOL_NAMES, ToolInputError, buildRequest, validateToolInput, serializeToolInput, callTool, createToolState};
});
