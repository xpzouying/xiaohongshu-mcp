'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'detail.js'), 'utf8');

function loadScript(overrides = {}) {
  const context = {
    URLSearchParams,
    URL,
    AbortController,
    FormData,
    location: {search: '', origin: 'http://ui.example.test'},
    document: {addEventListener() {}},
    window: {addEventListener() {}},
    XHS: {escapeHTML: value => String(value)},
    ...overrides
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  context.detailState = vm.runInContext('detailState', context);
  return context;
}

const context = loadScript();

test('受信任 HTTP 图片地址升级为 HTTPS', () => {
  assert.equal(context.safeImageURL('http://sns-webpic-qc.xhscdn.com/image.jpg'), 'https://sns-webpic-qc.xhscdn.com/image.jpg');
  assert.equal(context.safeImageURL('javascript:alert(1)'), '');
});

test('视频源优先选择 H264 并保留 HTTPS 备用地址', () => {
  const sources = context.videoSources({media:{stream:{
    h265:[{masterUrl:'http://video.example/h265.mp4', codec:'h265'}],
    h264:[{masterUrl:'http://video.example/h264.mp4', backupUrls:['http://backup.example/h264.mp4'], codec:'h264', default:true}]
  }}});
  assert.deepEqual([...sources.map(source => source.url)], [
    'https://video.example/h264.mp4',
    'https://backup.example/h264.mp4',
    'https://video.example/h265.mp4'
  ]);
});

function replyForm(values) {
  return {get: name => values[name]};
}

test('回复目标仅提供 comment_id 时接受', () => {
  assert.deepEqual(
    {...context.validateReply(replyForm({comment_id: ' comment-1 ', content: ' 回复 '}))},
    {comment_id: 'comment-1', user_id: '', content: '回复'}
  );
});

test('回复目标仅提供 user_id 时接受', () => {
  assert.deepEqual(
    {...context.validateReply(replyForm({user_id: ' user-1 ', content: ' 回复 '}))},
    {comment_id: '', user_id: 'user-1', content: '回复'}
  );
});

test('回复目标支持同时提供 comment_id 与 user_id', () => {
  assert.deepEqual(
    {...context.validateReply(replyForm({comment_id: ' comment-1 ', user_id: ' user-1 ', content: ' 回复 '}))},
    {comment_id: 'comment-1', user_id: 'user-1', content: '回复'}
  );
});

test('回复目标未提供 comment_id 与 user_id 时拒绝', () => {
  assert.equal(context.validateReply(replyForm({content: '回复'})), null);
});

test('回复内容为空白时拒绝', () => {
  assert.equal(context.validateReply(replyForm({comment_id: 'comment-1', content: ' '})), null);
});

function createDetailDOM({loadAll, clickReplies, limit, replyLimit, scrollSpeed}) {
  const listeners = {};
  const control = (name, value = '') => ({name, value, checked: false, disabled: false, addEventListener() {}});
  const controls = {
    load_all_comments: control('load_all_comments'),
    click_more_replies: control('click_more_replies'),
    limit: control('limit', String(limit)),
    reply_limit: control('reply_limit', String(replyLimit)),
    scroll_speed: control('scroll_speed', scrollSpeed),
    submit: control('', '')
  };
  controls.load_all_comments.checked = loadAll;
  controls.click_more_replies.checked = clickReplies;
  const form = {
    id: 'comment-options-form', elements: controls,
    addEventListener() {},
    querySelectorAll() { return Object.values(controls); }
  };
  const generic = {
    hidden: false, disabled: false, textContent: '', innerHTML: '',
    addEventListener() {}, querySelectorAll() { return []; }
  };
  const nodes = new Map([
    ['#comment-options-form', form], ['#cancel-detail', {...generic}],
    ['#detail-error', {...generic}], ['#detail-content', {...generic}],
    ['#comment-form', {...generic}], ['#comments', {...generic}],
    ['#reply-form', {...generic}], ['#cancel-reply', {...generic}]
  ]);
  return {
    controls,
    document: {
      addEventListener(type, callback) { listeners[type] = callback; },
      querySelector(selector) { return nodes.get(selector) || {...generic}; }
    },
    ready() { listeners.DOMContentLoaded(); }
  };
}

class BrowserFormData {
  constructor(form) {
    this.values = new Map();
    for (const field of Object.values(form.elements)) {
      if (!field.name || field.disabled) continue;
      if (field.name === 'load_all_comments' || field.name === 'click_more_replies') {
        if (field.checked) this.values.set(field.name, 'on');
      } else {
        this.values.set(field.name, field.value);
      }
    }
  }
  get(name) { return this.values.get(name) ?? null; }
}

test('首次加载结束后仍按 load_all_comments=false 禁用依赖控件', () => {
  const dom = createDetailDOM({loadAll:false, clickReplies:false, limit:20, replyLimit:10, scrollSpeed:'normal'});
  const ctx = loadScript({document:dom.document, FormData:BrowserFormData, XHS:{requireAccount:() => false}});
  dom.ready();
  ctx.setPending(true);
  ctx.setPending(false);
  assert.equal(dom.controls.limit.disabled, true);
  assert.equal(dom.controls.click_more_replies.disabled, true);
  assert.equal(dom.controls.reply_limit.disabled, true);
  assert.equal(dom.controls.scroll_speed.disabled, true);
});

test('pending 中重复加载保留五个高级参数', async () => {
  const dom = createDetailDOM({loadAll:true, clickReplies:true, limit:37, replyLimit:8, scrollSpeed:'slow'});
  const calls = [];
  const XHS = {
    requireAccount:() => true, loading() {}, toast() {},
    callTool: (_tool, input, options) => {
      calls.push(input);
      if (calls.length === 2) return Promise.resolve({data:{note:{}, comments:{list:[]}}});
      return new Promise((_resolve, reject) => options.signal.addEventListener('abort', () => reject(Object.assign(new Error('aborted'), {name:'AbortError'})), {once:true}));
    }
  };
  const ctx = loadScript({
    document:dom.document, FormData:BrowserFormData, XHS,
    location:{search:'?feed_id=feed-1&xsec_token=token-1'}
  });
  dom.ready();
  const first = ctx.loadDetail();
  const second = ctx.loadDetail();
  assert.deepEqual({...calls[1]}, {
    feed_id:'feed-1', xsec_token:'token-1', load_all_comments:true,
    click_more_replies:true, limit:37, reply_limit:8, scroll_speed:'slow'
  });
  await Promise.all([first, second]);
});