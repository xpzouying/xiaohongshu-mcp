'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'accounts.js'), 'utf8');

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((onResolve, onReject) => {
    resolve = onResolve;
    reject = onReject;
  });
  return {promise, resolve, reject};
}

function loadScript(callTool) {
  const timers = new Map();
  const listeners = {};
  let nextTimer = 1;
  const status = {textContent: ''};
  const dialog = {
    open: false,
    querySelector(selector) {
      if (selector === '.qr-status') return status;
      if (selector === 'img') return {src: ''};
      if (selector === 'button') return {addEventListener() {}};
      throw new Error(`unexpected selector ${selector}`);
    },
    addEventListener(type, callback) { listeners[type] = callback; },
    showModal() { this.open = true; },
    close() {
      this.open = false;
      listeners.close?.();
    }
  };
  const document = {
    querySelector(selector) {
      if (selector === '#qr-dialog') return dialog;
      return {addEventListener() {}};
    },
    addEventListener(type, callback) { listeners[type] = callback; }
  };
  const toasts = [];
  const context = {
    AbortController,
    FormData,
    confirm: () => true,
    document,
    window: {addEventListener() {}},
    setTimeout(callback) {
      const id = nextTimer++;
      timers.set(id, callback);
      return id;
    },
    clearTimeout(id) { timers.delete(id); },
    XHS: {
      callTool,
      escapeHTML: value => String(value),
      toast(message, type) { toasts.push({message, type}); }
    }
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  listeners.DOMContentLoaded?.();
  return {
    context,
    dialog,
    status,
    toasts,
    async runNextTimer() {
      const entry = timers.entries().next().value;
      assert.ok(entry, 'expected a pending timer');
      timers.delete(entry[0]);
      await entry[1]();
    },
    timerCount: () => timers.size
  };
}

test('身份对象显示真实昵称并兼容空 identity', () => {
  const {context} = loadScript(async () => ({}));
  assert.equal(context.loginIdentityName({nickname: '真实昵称'}), '真实昵称');
  assert.equal(context.loginIdentityName({display_name: '展示名'}), '展示名');
  assert.equal(context.loginIdentityName(null), '');
  assert.equal(context.loginIdentityName('旧版昵称'), '旧版昵称');
});

test('慢状态请求严格串行，完成后才安排下一次轮询', async () => {
  const requests = [];
  const first = deferred();
  const ui = loadScript((_tool, _input, options) => {
    requests.push(options);
    return requests.length === 1 ? first.promise : Promise.resolve({is_logged_in: false});
  });
  ui.context.showQRDialog('image', 'acct_one');
  assert.equal(ui.timerCount(), 1);
  const running = ui.runNextTimer();
  assert.equal(requests.length, 1);
  assert.equal(ui.timerCount(), 0, '请求未结束时不得安排或发起下一轮');
  first.resolve({is_logged_in: false});
  await running;
  assert.equal(ui.timerCount(), 1, '请求结束后才安排下一轮');
  await ui.runNextTimer();
  assert.equal(requests.length, 2);
});

test('关闭二维码 dialog 会取消进行中请求且不再轮询', async () => {
  const request = deferred();
  let calls = 0;
  const ui = loadScript((_tool, _input, options) => {
    calls++;
    options.signal.addEventListener('abort', () => request.reject(Object.assign(new Error('aborted'), {name: 'AbortError'})), {once: true});
    return request.promise;
  });
  ui.context.showQRDialog('image', 'acct_one');
  const running = ui.runNextTimer();
  ui.dialog.close();
  await running;
  assert.equal(calls, 1);
  assert.equal(ui.timerCount(), 0);
  assert.equal(ui.dialog.open, false);
});

test('连续轮询错误可见，并在达到上限后停止', async () => {
  let calls = 0;
  const ui = loadScript(async () => {
    calls++;
    throw new Error(`状态失败${calls}`);
  });
  ui.context.showQRDialog('image', 'acct_one');
  await ui.runNextTimer();
  assert.match(ui.status.textContent, /状态失败1/);
  assert.equal(ui.timerCount(), 1);
  await ui.runNextTimer();
  assert.equal(ui.timerCount(), 1);
  await ui.runNextTimer();
  assert.equal(calls, 3);
  assert.equal(ui.timerCount(), 0, '最终错误后必须停止，不能永久吞错重试');
  assert.match(ui.status.textContent, /已停止自动检查/);
  assert.ok(ui.toasts.some(item => item.type === 'error' && /状态失败3/.test(item.message)));
});
