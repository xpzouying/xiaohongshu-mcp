async function loadDashboard() {
  const health = document.querySelector('#health-state');
  try {
    const data = await XHS.api('/api/web/health', {account: false});
    health.innerHTML = `<span class="status-dot status-active"></span>${XHS.escapeHTML(data.status === 'healthy' ? '服务正常' : data.status)}`;
    document.querySelector('#service-name').textContent = data.service || 'xiaohongshu-mcp-webui';
  } catch (error) { health.textContent = error.message; health.className = 'metric error-text'; }
  const account = XHS.currentAccount();
  document.querySelector('#default-account').textContent = account ? `${account.display_name} (${account.id})` : '尚未选择账号';
  document.querySelector('#account-status').textContent = account?.status || '—';
}
window.addEventListener('accountsready', loadDashboard);
window.addEventListener('accountchange', loadDashboard);
document.addEventListener('DOMContentLoaded', loadDashboard);
