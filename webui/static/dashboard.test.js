'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'dashboard.js'), 'utf8');

async function renderIdentity(identity) {
  const nodes = new Map([
    ['#health-state', {innerHTML:'', textContent:'', className:''}],
    ['#service-name', {textContent:''}],
    ['#default-account', {textContent:''}],
    ['#account-status', {textContent:''}]
  ]);
  const listeners = [];
  const context = {
    document: {
      querySelector: selector => nodes.get(selector),
      addEventListener(_type, callback) { listeners.push(callback); }
    },
    window: {addEventListener(_type, callback) { listeners.push(callback); }},
    XHS: {
      api: async () => ({status:'healthy'}),
      currentAccount: () => ({id:'acct_one', display_name:'账号一', status:'active'}),
      callTool: async () => ({is_logged_in:true, identity}),
      escapeHTML: value => String(value)
    }
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  await context.loadDashboard();
  return nodes;
}

test('概览页显示 identity 真实昵称', async () => {
  const nodes = await renderIdentity({nickname:'真实昵称'});
  assert.equal(nodes.get('#default-account').textContent, '真实昵称');
  assert.equal(nodes.get('#account-status').textContent, '已登录');
});

test('概览页兼容空 identity 并保留账号展示名', async () => {
  const nodes = await renderIdentity(null);
  assert.equal(nodes.get('#default-account').textContent, '账号一');
  assert.equal(nodes.get('#account-status').textContent, '已登录');
});