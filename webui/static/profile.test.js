'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'profile.js'), 'utf8');

function createNode() {
  const listeners = new Map();
  return {
    innerHTML:'', textContent:'', hidden:false,
    addEventListener(type, listener) { listeners.set(type, listener); },
    click() { return listeners.get('click')?.({preventDefault() {}}); }
  };
}

function loadScript(overrides = {}) {
  const nodes = new Map([
    ['#profile-content', createNode()],
    ['#profile-feeds', createNode()],
    ['#note-count', createNode()],
    ['#profile-more', createNode()]
  ]);
  const calls = [];
  const XHS = {
    escapeHTML:value => String(value),
    requireAccount:() => true,
    loading() {},
    toast() {},
    async callTool(name, input) { calls.push({name, input}); return {data:{feeds:[]}}; },
    ...overrides
  };
  const context = {
    URLSearchParams,
    URL,
    location:{search:'?user_id=user-1&xsec_token=token-1', origin:'http://ui.example.test'},
    document:{querySelector: selector => nodes.get(selector)},
    window:{addEventListener() {}},
    XHS
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  return {context, nodes, calls};
}

function makeFeeds(count) {
  return Array.from({length:count}, (_, index) => ({
    id:`feed-${index}`, xsecToken:`token-${index}`,
    noteCard:{displayTitle:`标题${index}`, cover:{urlDefault:`http://img.example/${index}.jpg`}, interactInfo:{}}
  }));
}

function renderedCount(nodes) {
  return (nodes.get('#profile-feeds').innerHTML.match(/class="feed-card"/g) || []).length;
}

test('0 篇笔记展示空状态和准确计数', () => {
  const {context, nodes} = loadScript();
  context.renderProfileFeeds([]);
  assert.equal(nodes.get('#note-count').textContent, '共 0 篇');
  assert.match(nodes.get('#profile-feeds').innerHTML, /暂未发布笔记/);
  assert.equal(nodes.get('#profile-more').hidden, true);
});

test('少量笔记一次展示且图片保持 lazy load', () => {
  const {context, nodes} = loadScript();
  context.renderProfileFeeds(makeFeeds(5));
  assert.equal(nodes.get('#note-count').textContent, '共 5 篇');
  assert.equal(renderedCount(nodes), 5);
  assert.match(nodes.get('#profile-feeds').innerHTML, /loading="lazy"/);
  assert.equal(nodes.get('#profile-more').hidden, true);
});

test('较大列表首屏只展示一段并保留总数', () => {
  const {context, nodes} = loadScript();
  context.renderProfileFeeds(makeFeeds(30));
  assert.equal(nodes.get('#note-count').textContent, '已显示 12 / 共 30 篇');
  assert.equal(renderedCount(nodes), 12);
  assert.equal(nodes.get('#profile-more').hidden, false);
  assert.match(nodes.get('#profile-more').textContent, /再加载 12 篇/);
  assert.doesNotMatch(nodes.get('#profile-feeds').innerHTML, /标题29/);
});

test('加载更多可持续展示直至全部完成', () => {
  const {context, nodes} = loadScript();
  context.renderProfileFeeds(makeFeeds(30));
  nodes.get('#profile-more').click();
  assert.equal(renderedCount(nodes), 24);
  assert.equal(nodes.get('#note-count').textContent, '已显示 24 / 共 30 篇');
  nodes.get('#profile-more').click();
  assert.equal(renderedCount(nodes), 30);
  assert.equal(nodes.get('#note-count').textContent, '共 30 篇');
  assert.equal(nodes.get('#profile-more').hidden, true);
  assert.match(nodes.get('#profile-feeds').innerHTML, /https:\/\/img\.example\/29\.jpg/);
});

test('加载错误可在页面内重试且成功后不叠加旧数据', async () => {
  let attempt = 0;
  const {context, nodes, calls} = loadScript({
    async callTool() {
      attempt += 1;
      if (attempt === 1) throw new Error('临时失败');
      return {data:{feeds:makeFeeds(2)}};
    }
  });

  await context.loadProfile();
  assert.match(nodes.get('#profile-content').innerHTML, /临时失败/);
  assert.match(nodes.get('#profile-content').innerHTML, /data-action="retry-profile"/);
  assert.equal(renderedCount(nodes), 0);

  await context.retryProfile({preventDefault() {}});
  assert.equal(attempt, 2);
  assert.equal(renderedCount(nodes), 2);
  assert.equal(nodes.get('#note-count').textContent, '共 2 篇');
  assert.equal(calls.length, 0);
});

test('账号切换后的新请求不会被旧请求结果覆盖', async () => {
  const pending = [];
  const {context, nodes} = loadScript({
    callTool() {
      return new Promise(resolve => pending.push(resolve));
    }
  });

  const oldRequest = context.loadProfile();
  const newRequest = context.loadProfile();
  pending[1]({data:{feeds:makeFeeds(3)}});
  await newRequest;
  assert.equal(renderedCount(nodes), 3);

  pending[0]({data:{feeds:makeFeeds(20)}});
  await oldRequest;
  assert.equal(renderedCount(nodes), 3);
  assert.equal(nodes.get('#note-count').textContent, '共 3 篇');
});