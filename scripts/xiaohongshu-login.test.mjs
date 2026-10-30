// Copyright 2026 Marvelooous
// SPDX-License-Identifier: Apache-2.0
import test from 'node:test';
import assert from 'node:assert/strict';
import vm from 'node:vm';
import { fillExpression, inspectPage } from './xiaohongshu-login.mjs';

function inputFixture() {
  class Input {
    constructor() { this.events = []; this.isConnected = true; this.tagName = 'INPUT'; }
    set value(value) { this.stored = value; }
    get value() { return this.stored; }
    focus() {}
    getClientRects() { return [{}]; }
    dispatchEvent(event) { this.events.push(event.type); }
  }
  const field = new Input();
  const context = {
    window: { __xhsTerminalInputs: [field] },
    HTMLInputElement: Input,
    Event: class { constructor(type) { this.type = type; } },
  };
  return { field, context };
}

test('input is treated as data even when it contains quotes and JavaScript', () => {
  const { field, context } = inputFixture();
  const text = `"'); globalThis.injected = true; //\n测试`;
  vm.runInNewContext(fillExpression(1, text), context);
  assert.equal(field.value, text);
  assert.equal(context.injected, undefined);
  assert.deepEqual(field.events, ['input', 'change']);
});

test('a stale or disabled input is rejected instead of receiving a code', () => {
  const { field, context } = inputFixture();
  field.isConnected = false;
  assert.throws(() => vm.runInNewContext(fillExpression(1, 'TEST-ONLY'), context), /输入框已变化/);
  field.isConnected = true;
  field.disabled = true;
  assert.throws(() => vm.runInNewContext(fillExpression(1, 'TEST-ONLY'), context), /输入框已变化/);
  assert.equal(field.value, undefined);
});

test('nested duplicate buttons collapse while distinct same-label buttons remain', () => {
  function button(text, children = []) {
    return {
      innerText: text,
      getClientRects: () => [{}],
      getAttribute: () => null,
      contains: child => children.includes(child),
    };
  }
  const actual = button('登录');
  const wrapper = button('登录', [actual]);
  const other = button('登录');
  const window = {};
  const info = vm.runInNewContext(inspectPage, {
    window,
    document: {
      title: 'Fixture', body: { innerText: '登录' },
      querySelectorAll: selector => selector.startsWith('input:not') ? [] : [wrapper, actual, other],
    },
    getComputedStyle: () => ({ visibility: 'visible' }),
  });
  assert.equal(info.buttons.length, 2);
  assert.equal(window.__xhsTerminalButtons[0], actual);
  assert.equal(window.__xhsTerminalButtons[1], other);
});
