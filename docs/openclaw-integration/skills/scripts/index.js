#!/usr/bin/env node

/**
 * 小红书 MCP Skill — OpenClaw 集成 v3
 *
 * 通过 mcporter CLI 调用 xiaohongshu-mcp 服务的全部 13 个工具。
 *
 * 前提条件：
 *   1. xiaohongshu-mcp 服务运行在 localhost:18060
 *   2. mcporter 已全局安装：npm install -g mcporter
 *   3. ~/.mcporter/mcporter.json 中配置了 xiaohongshu 服务器
 *
 * 使用方式：
 *   OpenClaw Agent 通过 exec 工具调用 mcporter，不需要直接运行此脚本。
 *   此脚本作为 OpenClaw Skill 的入口，提供工具元数据供 Agent 了解可用工具。
 *
 * 直接调用示例（调试用）：
 *   echo '{"tool":"xhs_list_feeds","args":{}}' | node index.js
 *   echo '{"tool":"xhs_search_notes","args":{"keyword":"AI"}}' | node index.js
 */

const { spawnSync } = require('child_process');

/**
 * 调用 mcporter CLI 执行 xiaohongshu MCP 工具
 * @param {string} tool - MCP 工具名（不含 xiaohongshu. 前缀）
 * @param {object} args - 工具参数
 * @returns {object} - 解析后的 JSON 结果
 */
function mcporterCall(tool, args = {}) {
  const argList = ['call', `xiaohongshu.${tool}`];

  for (const [key, value] of Object.entries(args)) {
    if (value === undefined || value === null) continue;
    if (typeof value === 'object') {
      argList.push(`${key}=${JSON.stringify(value)}`);
    } else {
      argList.push(`${key}=${value}`);
    }
  }

  const result = spawnSync('mcporter', argList, {
    encoding: 'utf8',
    timeout: 600000, // 10 分钟，浏览器操作可能较慢
  });

  if (result.status !== 0) {
    throw new Error(`mcporter error: ${(result.stderr || result.stdout || '').trim()}`);
  }

  const out = (result.stdout || '').trim();
  try {
    return JSON.parse(out);
  } catch {
    return { raw: out };
  }
}

// ============================================================
// 工具定义
// ============================================================

const tools = {
  // ---------- 账号管理 ----------

  xhs_check_login: {
    description: '检查小红书登录状态',
    params: {},
    run: () => mcporterCall('check_login_status'),
  },

  xhs_get_login_qrcode: {
    description: '获取小红书登录二维码（返回 Base64 图片）',
    params: {},
    run: () => mcporterCall('get_login_qrcode'),
  },

  xhs_delete_cookies: {
    description: '删除 cookies 重置登录状态',
    params: {},
    run: () => mcporterCall('delete_cookies'),
  },

  // ---------- 浏览与搜索 ----------

  xhs_list_feeds: {
    description: '获取小红书首页推荐 Feed（返回约 30-35 条，不支持 limit 参数）',
    params: {},
    run: () => mcporterCall('list_feeds'),
  },

  xhs_search_notes: {
    description: '搜索小红书笔记',
    params: {
      keyword: { type: 'string', required: true, description: '搜索关键词' },
      sort_by: { type: 'string', description: '排序方式' },
      note_type: { type: 'string', description: '笔记类型筛选' },
      publish_time: { type: 'string', description: '发布时间筛选' },
      search_scope: { type: 'string', description: '搜索范围' },
      location: { type: 'string', description: '地点筛选' },
    },
    run: (args) => {
      if (!args.keyword) throw new Error('keyword is required');
      const a = { keyword: args.keyword };
      if (args.sort_by) a.sort_by = args.sort_by;
      if (args.note_type) a.note_type = args.note_type;
      if (args.publish_time) a.publish_time = args.publish_time;
      if (args.search_scope) a.search_scope = args.search_scope;
      if (args.location) a.location = args.location;
      return mcporterCall('search_feeds', a);
    },
  },

  xhs_get_note_detail: {
    description: '获取笔记详情，包含正文、图片、互动数据和评论列表',
    params: {
      feed_id: { type: 'string', required: true, description: '笔记 ID，从 list_feeds 或 search_feeds 结果中获取' },
      xsec_token: { type: 'string', required: true, description: '访问令牌，从 Feed 列表的 xsecToken 字段获取' },
      load_all_comments: { type: 'boolean', default: false, description: '是否加载全部评论（默认只返回前10条）' },
      limit: { type: 'number', description: '最多加载的一级评论数量（load_all_comments=true 时生效）' },
      click_more_replies: { type: 'boolean', description: '是否展开子评论（load_all_comments=true 时生效）' },
      reply_limit: { type: 'number', description: '跳过回复数超过此值的评论（click_more_replies=true 时生效）' },
      scroll_speed: { type: 'string', description: '滚动速度：slow/normal/fast' },
    },
    run: (args) => {
      if (!args.feed_id || !args.xsec_token) throw new Error('feed_id and xsec_token are required');
      const a = { feed_id: args.feed_id, xsec_token: args.xsec_token };
      if (args.load_all_comments) a.load_all_comments = true;
      if (args.limit !== undefined) a.limit = args.limit;
      if (args.click_more_replies) a.click_more_replies = true;
      if (args.reply_limit !== undefined) a.reply_limit = args.reply_limit;
      if (args.scroll_speed) a.scroll_speed = args.scroll_speed;
      return mcporterCall('get_feed_detail', a);
    },
  },

  xhs_get_user_profile: {
    description: '获取用户主页信息（基本信息、关注/粉丝数、笔记列表）',
    params: {
      user_id: { type: 'string', required: true, description: '用户 ID，从 Feed 列表获取' },
      xsec_token: { type: 'string', required: true, description: '访问令牌' },
    },
    run: (args) => {
      if (!args.user_id || !args.xsec_token) throw new Error('user_id and xsec_token are required');
      return mcporterCall('user_profile', { user_id: args.user_id, xsec_token: args.xsec_token });
    },
  },

  // ---------- 互动操作 ----------

  xhs_like_note: {
    description: '点赞或取消点赞笔记（如已点赞则跳过，如未点赞则跳过取消）',
    params: {
      feed_id: { type: 'string', required: true },
      xsec_token: { type: 'string', required: true },
      unlike: { type: 'boolean', default: false, description: 'true 为取消点赞' },
    },
    run: (args) => {
      if (!args.feed_id || !args.xsec_token) throw new Error('feed_id and xsec_token are required');
      return mcporterCall('like_feed', {
        feed_id: args.feed_id,
        xsec_token: args.xsec_token,
        unlike: args.unlike || false,
      });
    },
  },

  xhs_favorite_note: {
    description: '收藏或取消收藏笔记',
    params: {
      feed_id: { type: 'string', required: true },
      xsec_token: { type: 'string', required: true },
      unfavorite: { type: 'boolean', default: false, description: 'true 为取消收藏' },
    },
    run: (args) => {
      if (!args.feed_id || !args.xsec_token) throw new Error('feed_id and xsec_token are required');
      return mcporterCall('favorite_feed', {
        feed_id: args.feed_id,
        xsec_token: args.xsec_token,
        unfavorite: args.unfavorite || false,
      });
    },
  },

  xhs_comment_note: {
    description: '在笔记下发表顶级评论',
    params: {
      feed_id: { type: 'string', required: true },
      xsec_token: { type: 'string', required: true },
      content: { type: 'string', required: true, description: '评论内容' },
    },
    run: (args) => {
      if (!args.feed_id || !args.xsec_token || !args.content) {
        throw new Error('feed_id, xsec_token, content are required');
      }
      return mcporterCall('post_comment_to_feed', {
        feed_id: args.feed_id,
        xsec_token: args.xsec_token,
        content: args.content,
      });
    },
  },

  xhs_reply_comment: {
    description: '回复笔记下的特定评论（楼中楼）',
    params: {
      feed_id: { type: 'string', required: true },
      xsec_token: { type: 'string', required: true },
      comment_id: { type: 'string', description: '目标评论 ID，从评论列表获取' },
      content: { type: 'string', required: true, description: '回复内容' },
      user_id: { type: 'string', description: '目标评论用户 ID（可选）' },
    },
    run: (args) => {
      if (!args.feed_id || !args.xsec_token || !args.content) {
        throw new Error('feed_id, xsec_token, content are required');
      }
      const a = { feed_id: args.feed_id, xsec_token: args.xsec_token, content: args.content };
      if (args.comment_id) a.comment_id = args.comment_id;
      if (args.user_id) a.user_id = args.user_id;
      return mcporterCall('reply_comment_in_feed', a);
    },
  },

  // ---------- 发布操作 ----------

  xhs_publish: {
    description: '发布图文笔记（至少需要1张图片）',
    params: {
      title: { type: 'string', required: true, description: '标题（最多20个中文字或英文单词）' },
      content: { type: 'string', required: true, description: '正文内容（不含 # 标签，标签用 tags 参数）' },
      images: { type: 'array', required: true, description: '图片路径数组，支持本地路径或 HTTP URL' },
      tags: { type: 'array', description: '话题标签数组，如 ["AI", "技术"]' },
      schedule_at: { type: 'string', description: '定时发布时间，ISO8601 格式，如 2024-01-20T10:30:00+08:00' },
    },
    run: (args) => {
      if (!args.title || !args.content || !args.images?.length) {
        throw new Error('title, content, images are required');
      }
      const a = {
        title: args.title,
        content: args.content,
        images: JSON.stringify(args.images),
        tags: JSON.stringify(args.tags || []),
      };
      if (args.schedule_at) a.schedule_at = args.schedule_at;
      return mcporterCall('publish_content', a);
    },
  },

  xhs_publish_video: {
    description: '发布视频笔记（仅支持本地单个视频文件）',
    params: {
      title: { type: 'string', required: true, description: '标题（最多20个中文字或英文单词）' },
      content: { type: 'string', required: true, description: '正文内容' },
      video: { type: 'string', required: true, description: '本地视频文件绝对路径' },
      tags: { type: 'array', description: '话题标签数组' },
      schedule_at: { type: 'string', description: '定时发布时间，ISO8601 格式' },
    },
    run: (args) => {
      if (!args.title || !args.content || !args.video) {
        throw new Error('title, content, video are required');
      }
      const a = {
        title: args.title,
        content: args.content,
        video: args.video,
        tags: JSON.stringify(args.tags || []),
      };
      if (args.schedule_at) a.schedule_at = args.schedule_at;
      return mcporterCall('publish_with_video', a);
    },
  },
};

// ============================================================
// 主入口
// ============================================================

function main() {
  let input;
  if (process.argv.length >= 3) {
    input = process.argv[2];
  } else {
    input = require('fs').readFileSync(0, 'utf8');
  }

  let request;
  try {
    request = JSON.parse(input);
  } catch {
    console.log(JSON.stringify({ success: false, error: 'Invalid JSON input' }));
    process.exit(1);
  }

  const { tool, args = {} } = request;
  const t = tools[tool];

  if (!t) {
    const available = Object.keys(tools).join(', ');
    console.log(JSON.stringify({
      success: false,
      error: `Unknown tool: ${tool}. Available tools: ${available}`,
    }));
    process.exit(1);
  }

  try {
    const result = t.run(args);
    console.log(JSON.stringify({ success: true, result }));
  } catch (err) {
    console.log(JSON.stringify({ success: false, error: err.message }));
    process.exit(1);
  }
}

if (require.main === module) {
  main();
}

// 导出工具元数据（供 OpenClaw 读取）
module.exports = {
  tools: Object.keys(tools).map(name => ({
    name,
    description: tools[name].description,
    params: tools[name].params,
  })),
};
