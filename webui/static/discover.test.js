'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'discover.js'), 'utf8');

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((yes, no) => { resolve = yes; reject = no; });
  return {promise, resolve, reject};
}

function loadScript(callTool = async () => ({feeds: []}), loading = () => {}) {
  const listeners = {document: {}, window: {}};
  const node = () => ({
    innerHTML: '', textContent: '', hidden: false, dataset: {},
    classList: {toggle() {}}, setAttribute() {}, addEventListener() {}
  });
  const nodes = new Map([
    ['#recommended-grid', node()], ['#search-grid', node()], ['#result-count', node()],
    ['#recommended-panel', node()], ['#search-panel', node()], ['#refresh-feeds', node()], ['#search-form', node()]
  ]);
  const document = {
    querySelector: selector => nodes.get(selector) || node(),
    querySelectorAll: () => [],
    addEventListener(type, callback) { listeners.document[type] = callback; }
  };
  const window = {addEventListener(type, callback) { listeners.window[type] = callback; }};
  const context = {
    AbortController, FormData, document, window,
    XHS: {requireAccount: () => true, callTool, loading, toast() {}, escapeHTML: value => String(value)}
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  context.discoverState = vm.runInContext('discoverState', context);
  return {context, nodes, listeners};
}

test('普通 504 timeout 显示普通错误和重试，不宣称安全验证', async () => {
  const error = Object.assign(new Error('搜索Feeds失败'), {
    status: 504, code: 'SEARCH_TIMEOUT',
    details: {stage: 'wait_initial_state', error: 'context deadline exceeded'}
  });
  const {context, nodes} = loadScript(async () => { throw error; });
  await context.runSearch({keyword: '测试', filters: {}});
  assert.match(nodes.get('#search-grid').innerHTML, /搜索失败/);
  assert.match(nodes.get('#search-grid').innerHTML, /data-retry="search"/);
  assert.doesNotMatch(nodes.get('#search-grid').innerHTML, /安全验证|不是页面或系统故障/);
  assert.equal(nodes.get('#result-count').textContent, '搜索失败');
});

test('明确 challenge details 显示安全验证文案', async () => {
  const error = Object.assign(new Error('搜索Feeds失败'), {
    status: 504, code: 'SEARCH_TIMEOUT', details: {stage: 'captcha_challenge'}
  });
  const {context, nodes} = loadScript(async () => { throw error; });
  await context.runSearch({keyword: '测试', filters: {}});
  assert.match(nodes.get('#search-grid').innerHTML, /搜索需要完成安全验证/);
  assert.equal(nodes.get('#result-count').textContent, '搜索需要安全验证');
});

test('搜索普通错误清除旧结果和旧计数', async () => {
  const {context, nodes} = loadScript(async () => { throw new Error('网络暂时不可用'); });
  nodes.get('#search-grid').innerHTML = '<article>旧结果</article>';
  nodes.get('#result-count').textContent = '共 99 条';
  await context.runSearch({keyword: '新搜索', filters: {}});
  assert.doesNotMatch(nodes.get('#search-grid').innerHTML, /旧结果/);
  assert.equal(nodes.get('#result-count').textContent, '搜索失败');
});

test('推荐流错误替换加载状态并提供重试', async () => {
  const {context, nodes} = loadScript(async () => { throw new Error('服务不可用'); });
  await context.loadRecommended();
  assert.match(nodes.get('#recommended-grid').innerHTML, /推荐内容加载失败/);
  assert.match(nodes.get('#recommended-grid').innerHTML, /data-retry="recommended"/);
  assert.equal(context.discoverState.recommendedLoaded, false);
});

test('并发搜索的旧响应不能覆盖新响应', async () => {
  const first = deferred();
  const second = deferred();
  let calls = 0;
  const {context, nodes} = loadScript(() => (++calls === 1 ? first.promise : second.promise));
  const oldRequest = context.runSearch({keyword: '旧', filters: {}});
  const newRequest = context.runSearch({keyword: '新', filters: {}});
  second.resolve({feeds: [{id: 'new', noteCard: {displayTitle: '新结果'}}], count: 1});
  await newRequest;
  first.resolve({feeds: [{id: 'old', noteCard: {displayTitle: '旧结果'}}], count: 1});
  await oldRequest;
  assert.match(nodes.get('#search-grid').innerHTML, /新结果/);
  assert.doesNotMatch(nodes.get('#search-grid').innerHTML, /旧结果/);
});

test('账号切换后旧搜索响应不能恢复旧账号结果', async () => {
  const pending = deferred();
  const {context, nodes, listeners} = loadScript(() => pending.promise);
  nodes.get('#recommended-panel').hidden = true;
  const oldRequest = context.runSearch({keyword: '旧账号', filters: {}});
  listeners.window.accountchange();
  pending.resolve({feeds: [{id: 'old', noteCard: {displayTitle: '旧账号结果'}}], count: 1});
  await oldRequest;
  assert.match(nodes.get('#search-grid').innerHTML, /输入关键词开始搜索/);
  assert.doesNotMatch(nodes.get('#search-grid').innerHTML, /旧账号结果/);
  assert.equal(nodes.get('#result-count').textContent, '等待搜索');
});

test('推荐与搜索交错完成时旧请求不会提前关闭全局 loading', async () => {
  const recommended = deferred();
  const searched = deferred();
  const loadingEvents = [];
  const {context} = loadScript((tool) => tool === 'list_feeds' ? recommended.promise : searched.promise,
    (show) => loadingEvents.push(show));
  const recommendedRequest = context.loadRecommended();
  const searchRequest = context.runSearch({keyword:'并发', filters:{}});
  recommended.resolve({feeds:[]});
  await recommendedRequest;
  assert.equal(loadingEvents.at(-1), true);
  searched.resolve({feeds:[]});
  await searchRequest;
  assert.equal(loadingEvents.at(-1), false);
});
