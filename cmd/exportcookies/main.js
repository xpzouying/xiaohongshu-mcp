#!/usr/bin/env node
/**
 * 从 OpenClaw Chrome 实例导出小红书 cookies
 * 输出为 xiaohongshu-mcp 可加载的 cookies.json
 */

const path = require('path');
const fs = require('fs');
const sqlite3 = require('sqlite3').verbose();

async function exportCookies() {
  const home = process.env.HOME || process.env.USERPROFILE;
  if (!home) {
    console.error('Cannot determine HOME directory');
    process.exit(1);
  }

  const dbPath = path.join(home, '.openclaw', 'browser', 'openclaw', 'Default', 'Cookies');
  if (!fs.existsSync(dbPath)) {
    console.error('OpenClaw Cookies DB not found:', dbPath);
    process.exit(1);
  }

  const outPath = process.env.COOKIES_PATH || path.join(process.cwd(), 'cookies.json');

  const db = new sqlite3.Database(dbPath);
  db.serialize(() => {
    db.all(`
      SELECT name, value, host_key, path, expires_utc, is_secure, is_httponly, same_site
      FROM cookies
      WHERE host_key LIKE '%xiaohongshu.com%' OR host_key LIKE '%xhscdn.com%'
    `, async (err, rows) => {
      if (err) {
        console.error('Query error:', err);
        process.exit(1);
        return;
      }

      const cookies = rows.map(r => {
        // Convert Chrome's WebKit time (microseconds since 1601-01-01) to Unix seconds
        let expires = null;
        if (r.expires_utc !== null && r.expires_utc !== undefined) {
          const unixMillis = (r.expires_utc / 10) - 11644473600000;
          expires = Math.floor(unixMillis / 1000);
        }

        let sameSite = '';
        switch (r.same_site) {
          case '0': sameSite = ''; break;
          case '1': sameSite = 'Strict'; break;
          case '2': sameSite = 'Lax'; break;
          case '3': sameSite = 'None'; break;
        }

        return {
          name: r.name,
          value: r.value,
          domain: r.host_key,
          path: r.path,
          expires: expires,
          httpOnly: !!r.is_httponly,
          secure: !!r.is_secure,
          sameSite: sameSite
        };
      }).filter(c => c.value); // only keep non-empty

      fs.writeFileSync(outPath, JSON.stringify(cookies, null, 2));
      console.log(`✅ Exported ${cookies.length} cookies to ${outPath}`);
      process.exit(0);
    });
  });
}

exportCookies();
