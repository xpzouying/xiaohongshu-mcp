#!/usr/bin/env node
/**
 * 使用 OpenClaw browser 命令获取当前小红书页面的 cookies，并转换为 xiaohongshu-mcp 格式
 */

const { spawnSync } = require('child_process');

function openclaw(...args) {
  const result = spawnSync('openclaw', ['browser', ...args], { encoding: 'utf8' });
  if (result.status !== 0) {
    throw new Error(`openclaw error: ${result.stderr || 'unknown'}`);
  }
  return result.stdout;
}

function main() {
  // 1. 打开小红书首页
  openclaw('open', 'https://www.xiaohongshu.com');
  openclaw('wait', '--load', 'domcontentloaded');

  // 2. 获取 cookies（通过 evaluate 返回 document.cookie 字符串）
  const cookieStr = openclaw('evaluate', '--fn', '() => document.cookie');
  const cookiePairs = cookieStr.trim().split(';').filter(Boolean);

  const cookies = cookiePairs.map(pair => {
    const [name, ...rest] = pair.trim().split('=');
    const value = rest.join('=');
    // 注意：document.cookie 不包含 domain/path/expires 等信息，但 MCP 的 WithCookies 只需要 name/value
    return {
      name: name.trim(),
      value: value.trim(),
      domain: '.xiaohongshu.com',
      path: '/',
      httpOnly: false, // 无法从 document.cookie 获取
      secure: true,
      sameSite: 'None'
    };
  }).filter(c => c.name);

  // 3. 输出为 JSON
  const out = JSON.stringify(cookies, null, 2);
  const outPath = process.env.COOKIES_PATH || 'cookies.json';
  require('fs').writeFileSync(outPath, out);
  console.log(`✅ Exported ${cookies.length} cookies (name/value only) to ${outPath}`);
}

try {
  main();
} catch (err) {
  console.error('❌', err.message);
  process.exit(1);
}
