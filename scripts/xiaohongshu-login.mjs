#!/usr/bin/env node
// Copyright 2026 Marvelooous
// SPDX-License-Identifier: Apache-2.0
// Linux terminal login helper for the accompanying xiaohongshu-mcp fork.
import { readFile, readdir, mkdtemp, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { spawnSync } from 'node:child_process';
import { createInterface } from 'node:readline/promises';
import { Writable } from 'node:stream';
import { pathToFileURL } from 'node:url';

const endpoint = process.env.XHS_MCP_URL || 'http://127.0.0.1:18060/mcp';
const token = process.env.XHS_MCP_AUTH_TOKEN || process.env.AUTH_TOKEN;
const requestHeaders = { 'Content-Type': 'application/json', Accept: 'application/json, text/event-stream' };
if (token) requestHeaders.Authorization = `Bearer ${token}`;
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));

export async function mcp(name) {
  const response = await fetch(endpoint, {
    method: 'POST',
    headers: requestHeaders,
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/call', params: { name, arguments: {} } }),
    signal: AbortSignal.timeout(180000),
  });
  if (!response.ok) throw new Error(`MCP HTTP ${response.status}`);
  const result = await response.json();
  if (result.error || result.result?.isError) {
    throw new Error(result.error?.message || result.result?.content?.find(x => x.type === 'text')?.text || 'MCP 调用失败');
  }
  return result.result;
}

export async function browserTab() {
  for (const pid of await readdir('/proc')) {
    if (!/^\d+$/.test(pid)) continue;
    try {
      const args = (await readFile(`/proc/${pid}/cmdline`, 'utf8')).split('\0');
      if (!args[0]?.endsWith('/chrome') || !args[0].includes('xiaohongshu-mcp/browser/')) continue;
      if (args.some(a => a.startsWith('--type='))) continue;
      const dir = args.find(a => a.startsWith('--user-data-dir='))?.slice(16);
      if (!dir) continue;
      const port = (await readFile(join(dir, 'DevToolsActivePort'), 'utf8')).split('\n')[0];
      const response = await fetch(`http://127.0.0.1:${port}/json/list`, { signal: AbortSignal.timeout(2000) });
      const tabs = await response.json();
      const tab = tabs.find(t => t.type === 'page' && /^https:\/\/([^/]*\.)?xiaohongshu\.com\//.test(t.url));
      if (tab) return tab;
    } catch { /* Browser may be starting or closing. */ }
  }
  throw new Error('未找到小红书登录页面；会话可能已过期，请退出后重新运行。');
}

export async function connect(url) {
  const socket = new WebSocket(url);
  const waiting = new Map();
  let nextId = 0;
  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      reject(new Error('连接浏览器页面超时'));
      socket.close();
    }, 10000);
    socket.addEventListener('open', () => { clearTimeout(timer); resolve(); }, { once: true });
    socket.addEventListener('error', () => {
      clearTimeout(timer);
      reject(new Error('无法连接浏览器页面'));
    }, { once: true });
  });
  socket.addEventListener('message', event => {
    const message = JSON.parse(String(event.data));
    const item = waiting.get(message.id);
    if (!item) return;
    waiting.delete(message.id);
    clearTimeout(item.timer);
    if (message.error) item.reject(new Error(message.error.message));
    else item.resolve(message.result);
  });
  socket.addEventListener('close', () => {
    for (const item of waiting.values()) { clearTimeout(item.timer); item.reject(new Error('登录页面已关闭或会话已结束')); }
    waiting.clear();
  });
  return {
    get closed() { return socket.readyState === WebSocket.CLOSED; },
    call(method, params = {}) {
      return new Promise((resolve, reject) => {
        if (socket.readyState !== WebSocket.OPEN) return reject(new Error('浏览器连接已关闭'));
        const id = ++nextId;
        const timer = setTimeout(() => { waiting.delete(id); reject(new Error('页面操作超时，请稍后刷新重试')); }, 30000);
        waiting.set(id, { resolve, reject, timer });
        socket.send(JSON.stringify({ id, method, params }));
      });
    },
    close() { socket.close(); },
  };
}

export async function evaluate(cdp, expression) {
  const result = await cdp.call('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true });
  if (result.exceptionDetails) throw new Error(result.exceptionDetails.exception?.description || result.exceptionDetails.text || '页面操作失败');
  return result.result.value;
}

export const inspectPage = `(() => {
  const visible = e => e.getClientRects().length && getComputedStyle(e).visibility !== 'hidden';
  const label = e => (e.innerText || e.value || e.getAttribute('aria-label') || '').trim();
  window.__xhsTerminalInputs = [...document.querySelectorAll('input:not([type=hidden]),textarea')].filter(visible);
  const candidates = [...document.querySelectorAll('button,[role=button],input[type=submit],a,[class*=btn],[class*=button]')]
    .filter(e => visible(e) && label(e));
  window.__xhsTerminalButtons = candidates.filter(e =>
    !candidates.some(child => child !== e && e.contains(child) && label(child) === label(e))).slice(0,80);
  return {
    title: document.title,
    text: document.body.innerText.slice(0,1600),
    inputs: window.__xhsTerminalInputs.map((e,i) => ({ id:i+1, label:e.placeholder || e.getAttribute('aria-label') || e.name || e.type })),
    buttons: window.__xhsTerminalButtons.map((e,i) => ({ id:i+1, label:(e.innerText || e.value || e.getAttribute('aria-label') || '').trim().slice(0,55) }))
  };
})()`;

export function fillExpression(index, value) {
  return `(() => {
    const e = window.__xhsTerminalInputs?.[${index - 1}];
    if (!e || !e.isConnected || !e.getClientRects().length || e.disabled || e.readOnly) throw new Error('输入框已变化，请先刷新');
    e.focus();
    const proto = e.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto,'value').set.call(e,${JSON.stringify(value)});
    e.dispatchEvent(new Event('input',{bubbles:true}));
    e.dispatchEvent(new Event('change',{bubbles:true}));
    return true;
  })()`;
}

async function secretPrompt(prompt) {
  // A silent terminal interface keeps the code out of shell history and output.
  const silent = new Writable({ write(_chunk, _encoding, callback) { callback(); } });
  const reader = createInterface({ input: process.stdin, output: silent, terminal: true });
  process.stdout.write(prompt);
  try { return await reader.question(''); }
  finally { reader.close(); process.stdout.write('\n'); }
}

function showImage(path) {
  const args = ['--format=symbols', '--symbols=block', '--size=100x42'];
  const result = spawnSync('chafa', [...args, path], { stdio: 'inherit' });
  if (result.error || result.status !== 0) throw new Error('chafa 无法显示图片');
}

async function showQRCode(path) {
  const decoded = spawnSync('zbarimg', ['--quiet', '--raw', '--nodbus', path], { encoding: 'utf8' });
  if (decoded.status !== 0 || !decoded.stdout.trim()) throw new Error('无法解码原始二维码，请重新获取。');
  const payload = decoded.stdout.replace(/\r?\n$/, '');
  const output = spawnSync('qrencode', ['-t', 'ANSIUTF8', '-m', '4'], { input: payload, encoding: 'utf8' });
  if (output.status !== 0) throw new Error('二维码重新生成失败');
  const lines = output.stdout.split('\n').filter((line, i, list) => i < list.length - 1 || line);
  const width = Math.max(...lines.map(line => line.replace(/\x1b\[[0-9;]*m/g, '').length));
  const height = lines.length + 2;
  while ((process.stdout.columns || 80) < width || (process.stdout.rows || 24) < height) {
    console.log(`二维码需要至少 ${width} 列 × ${height} 行；当前终端 ${process.stdout.columns} 列 × ${process.stdout.rows} 行。`);
    console.log('请缩小终端字体或放大窗口，避免二维码被换行或截断。');
    const reader = createInterface({ input: process.stdin, output: process.stdout });
    try { await reader.question('调整后按回车继续 > '); } finally { reader.close(); }
  }
  const png = join(dirname(path), 'qr-large.png');
  const encoded = spawnSync('qrencode', ['-t', 'PNG', '-s', '10', '-m', '4', '-o', '-'], { input: payload });
  if (encoded.status !== 0) throw new Error('高清二维码生成失败');
  await writeFile(png, encoded.stdout, { mode: 0o600 });
  console.log('请用小红书 App 扫码；收到验证码后按回车，进入网页操作。');
  // Render exact QR modules. Never rescale a QR code as generic character art.
  process.stdout.write('\x1b[2J\x1b[H' + output.stdout);
  const reader = createInterface({ input: process.stdin, output: process.stdout });
  try { await reader.question('扫码后按回车 > '); } finally { reader.close(); }
  console.log('高清二维码文件（退出后自动删除）：', png);
}

async function reportLoginStatus() {
  console.log('浏览器会话已结束，正在通过 MCP 核对保存的登录状态……');
  const result = await mcp('check_login_status');
  for (const item of result.content || []) if (item.type === 'text') console.log(item.text);
}

async function main() {
  if (process.platform !== 'linux') throw new Error('此助手使用 Linux /proc，只支持在 MCP 所在 Linux 主机上运行。');
  if (typeof WebSocket !== 'function') throw new Error('请使用 Node.js 22 或更高版本（推荐 Node.js 24）。');
  if (!process.stdin.isTTY) throw new Error('请在自己的交互式终端中运行这个助手。');
  if (spawnSync('chafa', ['--version'], { stdio: 'ignore' }).status !== 0) throw new Error('请先安装 chafa。');
  for (const tool of ['qrencode', 'zbarimg']) {
    if (spawnSync(tool, ['--version'], { stdio: 'ignore' }).error) throw new Error(`缺少 ${tool}`);
  }
  const temp = await mkdtemp(join(tmpdir(), 'xhs-terminal-login-'));
  let cdp;
  try {
    console.log('正在请求登录二维码，页面加载可能需要一两分钟……');
    const result = await mcp('get_login_qrcode');
    let hasQRCode = false;
    for (const item of result.content || []) {
      if (item.type === 'text') console.log(item.text);
      if (item.type === 'image') {
        hasQRCode = true;
        const path = join(temp, 'qr.png');
        await writeFile(path, Buffer.from(item.data, 'base64'), { mode: 0o600 });
        await showQRCode(path);
      }
    }
    // An already logged-in account returns text and closes the browser.
    if (!hasQRCode) return;
    let tab;
    try { tab = await browserTab(); }
    catch { await reportLoginStatus(); return; }
    cdp = await connect(tab.webSocketDebuggerUrl);
    console.log('\n用小红书 App 扫码。需要验证码时，按回车查看网页和输入框。');
    console.log('命令：回车=刷新；i 编号=填写输入框；b 编号=点击按钮；q=退出。');
    console.log('验证码会在本终端隐藏输入，请勿发送到聊天中。');
    let refresh = true;
    while (true) {
      if (cdp.closed) { await reportLoginStatus(); return; }
      if (refresh) {
        try {
          const info = await evaluate(cdp, inspectPage);
          console.log('\n页面：', info.title, '\n', info.text);
          for (const input of info.inputs) console.log(`输入框 ${input.id}: ${input.label}`);
          for (const button of info.buttons) console.log(`按钮 ${button.id}: ${button.label}`);
          const shot = await cdp.call('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false });
          const path = join(temp, 'page.png');
          await writeFile(path, Buffer.from(shot.data, 'base64'), { mode: 0o600 });
          showImage(path);
        } catch (error) {
          if (cdp.closed) { await reportLoginStatus(); return; }
          console.log('页面暂未就绪：', error.message, '。可按回车重试，或输入 q 退出。');
        }
      }
      refresh = false;
      const reader = createInterface({ input: process.stdin, output: process.stdout });
      let command;
      try { command = (await reader.question('操作 > ')).trim(); }
      finally { reader.close(); }
      if (command === 'q') break;
      if (!command) { refresh = true; continue; }
      const match = command.match(/^([ib])\s+([1-9]\d*)$/);
      if (!match) { console.log('请输入 i 1、b 1、回车或 q。'); continue; }
      try {
        const index = Number(match[2]);
        if (match[1] === 'i') {
          let value = await secretPrompt(`输入框 ${index} 的内容（隐藏输入）: `);
          await evaluate(cdp, fillExpression(index, value));
          value = '';
          console.log('已填写。输入 b 加按钮编号提交；回车查看最新页面。');
        } else {
          await evaluate(cdp, `(() => {
            const e=window.__xhsTerminalButtons?.[${index - 1}];
            if(!e || !e.isConnected || !e.getClientRects().length || e.disabled) throw new Error('按钮已变化，请刷新');
            e.click(); return true;
          })()`);
          await pause(1000);
          refresh = true;
        }
      } catch (error) { console.log(error.message); }
    }
    console.log('已退出终端助手。请让 MCP 完成登录保存，再检查登录状态。');
  } finally {
    cdp?.close();
    await rm(temp, { recursive: true, force: true });
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch(error => { console.error('登录助手：', error.message); process.exitCode = 1; });
}
