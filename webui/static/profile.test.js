'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const source = fs.readFileSync(path.join(__dirname, 'profile.js'), 'utf8');

function loadScript() {
  const nodes = new Map([
    ['#profile-content', {innerHTML:''}],
    ['#profile-feeds', {innerHTML:''}],
    ['#note-count', {textContent:''}]
  ]);
  const context = {
    URLSearchParams,
    URL,
    location:{search:'', origin:'http://ui.example.test'},
    document:{querySelector: selector => nodes.get(selector)},
    window:{addEventListener() {}},
    XHS:{escapeHTML:value => String(value)}
  };
  vm.createContext(context);
  vm.runInContext(source, context);
  return {context, nodes};
}

test('HTTP 封面升级 HTTPS 并渲染全部 9 篇笔记', () => {
  const {context, nodes} = loadScript();
  const feeds = Array.from({length:9}, (_, index) => ({
    id:`feed-${index}`, xsecToken:`token-${index}`,
    noteCard:{displayTitle:`标题${index}`, cover:{urlDefault:`http://img.example/${index}.jpg`}, interactInfo:{}}
  }));
  context.renderProfileFeeds(feeds);
  assert.equal(nodes.get('#note-count').textContent, '共 9 篇');
  assert.equal((nodes.get('#profile-feeds').innerHTML.match(/class="feed-card"/g) || []).length, 9);
  assert.match(nodes.get('#profile-feeds').innerHTML, /https:\/\/img\.example\/8\.jpg/);
});