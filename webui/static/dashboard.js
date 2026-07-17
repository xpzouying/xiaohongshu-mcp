async function loadDashboard() {
  const health = document.querySelector('#health-state');
  try {
    const data = await XHS.api('/api/web/health', {account: false});
    health.innerHTML = `<span class="status-dot status-active"></span>${XHS.escapeHTML(data.status === 'healthy' ? '服务正常' : data.status)}`;
    document.querySelector('#service-name').textContent = data.service || 'xiaohongshu-mcp-webui';
  } catch (error) { health.textContent = error.message; health.className = 'metric error-text'; }
  // 先用 list_accounts 的结果填充展示名
  const account = XHS.currentAccount();
  const accountEl = document.querySelector('#default-account');
  const statusEl = document.querySelector('#account-status');
  if (account) {
    accountEl.textContent = account.display_name || account.id;
    statusEl.textContent = account.status || '—';
  } else {
    accountEl.textContent = '尚未选择账号';
    statusEl.textContent = '—';
  }
  // 尝试拉取当前账号的真实身份信息（登录状态/昵称）
  if (account && account.id) {
    try {
      const status = await XHS.callTool('check_login_status', {account_id: account.id});
      const identity = status.identity;
      const nickname = typeof identity === 'string' ? identity : identity?.nickname || identity?.display_name || '';
      if (nickname) accountEl.textContent = nickname;
      statusEl.textContent = status.is_logged_in ? '已登录' : '需登录';
    } catch (_) { /* 身份查询失败不阻塞概览 */ }
  }
}
window.addEventListener('accountsready', loadDashboard);
window.addEventListener('accountchange', loadDashboard);
document.addEventListener('DOMContentLoaded', loadDashboard);
