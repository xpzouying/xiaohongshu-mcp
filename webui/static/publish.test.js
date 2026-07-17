'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'publish.js'), 'utf8');

function escapeHTML(value = '') {
  return String(value).replace(/[&<>'"]/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));
}

function loadScript(overrides = {}) {
  const context = {
    URL,
    document: {addEventListener() {}},
    window: {addEventListener() {}},
    XHS: {
      state: {selectedAccountId:'account-1'},
      currentAccount:() => ({id:'account-1', display_name:'测试账号'}),
      escapeHTML,
      loading() {},
      toast() {}
    },
    ...overrides
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  return context;
}

const imageBody = {
  title:'图文标题', content:'第一行\n第二行', tags:['标签一'], products:['商品一'],
  visibility:'仅自己可见', schedule_at:'', images:['https://example.test/a.jpg'], is_original:false
};
const videoBody = {
  title:'视频标题', content:'视频正文', tags:[], products:[], visibility:'公开可见',
  schedule_at:'2026-07-20T04:00:00.000Z', video:'/srv/media/demo.MP4'
};

test('确认摘要转义用户输入，避免 XSS 注入', () => {
  const context = loadScript();
  const body = {...imageBody, title:'<img src=x onerror=alert(1)>', content:'正文 <script>alert(1)</script>'};
  const html = context.confirmationHTML(body, 'image', '账号 <admin>');
  assert.doesNotMatch(html, /<script>|<img src=/);
  assert.match(html, /&lt;script&gt;alert\(1\)&lt;\/script&gt;/);
  assert.match(html, /账号 &lt;admin&gt;/);
});

test('图文确认摘要包含正文、业务字段和图片数量', () => {
  const context = loadScript();
  const details = context.confirmationDetails(imageBody, 'image', '测试账号 (account-1)');
  assert.deepEqual([...details.map(item => [...item])], [
    ['当前账号','测试账号 (account-1)'], ['发布模式','图文笔记'], ['标题','图文标题'],
    ['正文摘要','第一行 第二行'], ['标签','标签一'], ['商品','商品一'],
    ['可见范围','仅自己可见'], ['发布时间','立即发布'], ['素材数量','1 张图片']
  ]);
});

test('视频确认摘要包含定时信息，路径信息包含文件名和扩展名', () => {
  const context = loadScript();
  const details = context.confirmationDetails(videoBody, 'video', '测试账号 (account-1)');
  assert.equal(details.find(([label]) => label === '发布模式')[1], '视频笔记');
  assert.equal(details.find(([label]) => label === '正文摘要')[1], '视频正文');
  assert.equal(details.find(([label]) => label === '发布时间')[1], '定时：2026-07-20T04:00:00.000Z');
  assert.equal(details.find(([label]) => label === '素材数量')[1], '1 个视频');
  assert.equal(context.localPathLabel(videoBody.video), 'demo.MP4');
  assert.equal(context.fileExtension(videoBody.video), '.mp4');
});

test('取消确认清空待发布数据且不发送请求', () => {
  let calls = 0, closes = 0;
  const dialog = {close() { closes += 1; }};
  const context = loadScript({
    document:{addEventListener() {}, querySelector: selector => selector === '#publish-confirmation' ? dialog : null},
    XHS:{state:{selectedAccountId:'account-1'}, currentAccount:() => null, escapeHTML, callTool:() => { calls += 1; }}
  });
  vm.runInContext(`pendingConfirmation = {body:${JSON.stringify(imageBody)}, mode:'image'}`, context);
  context.closeConfirmation();
  assert.equal(calls, 0);
  assert.equal(closes, 1);
  assert.equal(vm.runInContext('pendingConfirmation', context), null);
});

test('连续确认仅发送一次发布请求', async () => {
  let calls = 0, resolveCall;
  const call = new Promise(resolve => { resolveCall = resolve; });
  const button = {disabled:false};
  const dialog = {close() {}};
  const context = loadScript({
    document:{
      addEventListener() {},
      querySelector: selector => selector === '#confirm-publish' ? button : dialog
    },
    XHS:{
      state:{selectedAccountId:'account-1'}, currentAccount:() => null, escapeHTML,
      loading() {}, toast() {}, callTool:() => { calls += 1; return call; }
    }
  });
  vm.runInContext(`pendingConfirmation = {body:${JSON.stringify(videoBody)}, mode:'video', accountId:'account-1', accountLabel:'测试账号 (account-1)'}`, context);
  const first = context.confirmPublish();
  const second = context.confirmPublish();
  assert.equal(calls, 1);
  assert.equal(button.disabled, true);
  resolveCall({status:'ok'});
  await Promise.all([first, second]);
  assert.equal(calls, 1);
  assert.equal(button.disabled, false);
});

test('确认后切换账号会阻止发布并要求重新确认', async () => {
  let calls = 0, closes = 0;
  const summary = {innerHTML:''};
  const media = {replaceChildren() {}, append() {}};
  const dialog = {showModal() {}, close() { closes += 1; }};
  const button = {disabled:false};
  const context = loadScript({
    document:{
      addEventListener() {}, createElement:() => ({className:'', textContent:'', setAttribute() {}}),
      querySelector: selector => ({
        '#confirmation-summary':summary, '#confirmation-media':media,
        '#publish-confirmation':dialog, '#confirm-publish':button
      })[selector]
    },
    XHS:{
      state:{selectedAccountId:'account-1'}, currentAccount:() => ({id:'account-1', display_name:'账号一'}),
      escapeHTML, loading() {}, toast() {}, callTool:() => { calls += 1; }
    }
  });
  context.openConfirmation(videoBody, 'video');
  context.XHS.state.selectedAccountId = 'account-2';
  await context.confirmPublish();
  assert.equal(calls, 0);
  assert.equal(closes, 1);
  assert.equal(vm.runInContext('pendingConfirmation', context), null);
});

test('确认期间账号切走再切回仍需重新确认', async () => {
  let calls = 0, closes = 0, warnings = 0;
  const dialog = {close() { closes += 1; }};
  const context = loadScript({
    document:{addEventListener() {}, querySelector: () => dialog},
    XHS:{
      state:{selectedAccountId:'account-1'}, currentAccount:() => null, escapeHTML,
      loading() {}, toast() { warnings += 1; }, callTool:() => { calls += 1; }
    }
  });
  vm.runInContext(`pendingConfirmation = {body:${JSON.stringify(videoBody)}, mode:'video', accountId:'account-1'}`, context);
  context.XHS.state.selectedAccountId = 'account-2';
  context.invalidateConfirmationOnAccountChange();
  context.XHS.state.selectedAccountId = 'account-1';
  await context.confirmPublish();
  assert.equal(calls, 0);
  assert.equal(closes, 1);
  assert.equal(warnings, 1);
  assert.equal(vm.runInContext('pendingConfirmation', context), null);
});

for (const scenario of [
  ['失败', Object.assign(new Error('发布失败'), {code:'UPSTREAM_UNAVAILABLE'})],
  ['超时 UNKNOWN', Object.assign(new Error('请求超时'), {code:'REQUEST_TIMEOUT', status:504})]
]) {
  test(`${scenario[0]}后再次确认不会发送额外请求`, async () => {
    let calls = 0, closes = 0;
    const button = {disabled:false};
    const dialog = {close() { closes += 1; }};
    const context = loadScript({
      document:{addEventListener() {}, querySelector: selector => selector === '#confirm-publish' ? button : dialog},
      XHS:{
        state:{selectedAccountId:'account-1'}, currentAccount:() => ({id:'account-1', display_name:'账号一'}),
        escapeHTML, loading() {}, toast() {}, callTool:async () => { calls += 1; throw scenario[1]; }
      }
    });
    vm.runInContext(`pendingConfirmation = {body:${JSON.stringify(videoBody)}, mode:'video', accountId:'account-1', accountLabel:'账号一 (account-1)'}`, context);
    await context.confirmPublish();
    await context.confirmPublish();
    assert.equal(calls, 1);
    assert.equal(closes, 1);
    assert.equal(vm.runInContext('pendingConfirmation', context), null);
  });
}
