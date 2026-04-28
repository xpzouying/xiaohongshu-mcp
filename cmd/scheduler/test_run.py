#!/usr/bin/env python3
"""
快速测试脚本：调用运行中的 MCP 服务 + DeepSeek + 飞书
关键词从 keywords.txt 读取，也可用 -k 参数覆盖

用法:
  python test_run.py
  python test_run.py -k 申论低分
"""
import argparse, datetime, os, requests, sys

MCP_URL    = os.getenv('MCP_URL',    'http://127.0.0.1:18060/mcp')
API_KEY    = os.getenv('DMXAPI_KEY', 'sk-Xj5R9SWrx5Nhj98TFR232rhYEeJsII5zreVMfPgtjss9jsBT')
FEISHU_URL = os.getenv('FEISHU_WEBHOOK_URL', 'https://www.feishu.cn/flow/api/trigger-webhook/be47125fa3cdf730bbb715fe84168f66')
DMXAPI     = 'https://www.dmxapi.cn/v1/chat/completions'

def load_keyword(override):
    if override:
        return override
    kw_file = os.path.join(os.path.dirname(__file__), 'keywords.txt')
    if os.path.exists(kw_file):
        for line in open(kw_file, encoding='utf-8'):
            line = line.strip()
            if line:
                return line
    print('未指定关键词，请在 keywords.txt 填写或用 -k 参数传入', file=sys.stderr)
    sys.exit(1)

def mcp_init():
    hdrs = {'Content-Type': 'application/json', 'Accept': 'application/json, text/event-stream'}
    r = requests.post(MCP_URL, headers=hdrs, json={
        'jsonrpc': '2.0', 'id': 0, 'method': 'initialize',
        'params': {'protocolVersion': '2024-11-05', 'capabilities': {}, 'clientInfo': {'name': 'scheduler', 'version': '1.0'}}
    }, timeout=30)
    sid = r.headers.get('Mcp-Session-Id', '')
    hdrs['Mcp-Session-Id'] = sid
    requests.post(MCP_URL, headers=hdrs, json={'jsonrpc': '2.0', 'method': 'notifications/initialized', 'params': {}}, timeout=10)
    return hdrs

def get_hot_feeds(hdrs, keyword, min_likes=100):
    r = requests.post(MCP_URL, headers=hdrs, json={
        'jsonrpc': '2.0', 'id': 1, 'method': 'tools/call',
        'params': {'name': 'get_hot_feeds', 'arguments': {'keyword': keyword, 'min_likes': min_likes, 'sort_by': '最多点赞'}}
    }, timeout=180)
    text = r.json()['result']['content'][0]['text']
    return text.split('---')[0].strip() if '---' in text else text[:2000]

def llm(model, prompt):
    ai_hdrs = {'Authorization': API_KEY, 'Content-Type': 'application/json', 'Accept': 'application/json'}
    r = requests.post(DMXAPI, headers=ai_hdrs, json={
        'model': model, 'messages': [{'role': 'user', 'content': prompt}]
    }, timeout=120)
    r.raise_for_status()
    return r.json()['choices'][0]['message']['content']

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('-k', '--keyword', default='', help='搜索关键词（覆盖 keywords.txt）')
    parser.add_argument('--min-likes', type=int, default=100, help='最低点赞数')
    args = parser.parse_args()

    keyword = load_keyword(args.keyword)
    print(f'关键词: {keyword}')

    print('Step 1: 初始化 MCP...')
    hdrs = mcp_init()

    print('Step 2: 获取热帖...')
    summary = get_hot_feeds(hdrs, keyword, args.min_likes)
    print(f'       {len(summary)} 字')

    print('Step 3: DeepSeek-V3.2-Thinking 分析...')
    analysis = llm('DeepSeek-V3.2-Thinking',
        f'分析"{keyword}"爆款帖子的标题规律、高互动原因、创作公式，400字以内：\n\n{summary}')
    print(f'       {len(analysis)} 字')

    print('Step 4: DeepSeek-V3.2 生成5篇帖子...')
    posts = llm('DeepSeek-V3.2',
        f'【爆款分析】\n{analysis}\n\n【参考数据】\n{summary}\n\n'
        f'围绕"{keyword}"创作5篇原创小红书帖子，格式用===帖子N===分隔，含标题/正文150-250字/标签：')
    print(f'       {len(posts)} 字')

    print('Step 5: 发送飞书...')
    today = datetime.date.today().strftime('%Y-%m-%d')
    r = requests.post(FEISHU_URL, json={
        'msg_type': 'text',
        'content': {
            'title':    f'小红书爆款分析 | {keyword} | {today}',
            'analysis': analysis,
            'posts':    posts,
        }
    })
    print(f'       响应: {r.text}')
    print(f'\n完成！分析:{len(analysis)}字 | 帖子:{len(posts)}字')

if __name__ == '__main__':
    main()
