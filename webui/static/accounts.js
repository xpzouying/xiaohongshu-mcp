let loginTimer;
const statusLabel = {active:'已登录', needs_login:'需登录', paused:'已暂停', risk_hold:'风控冻结', disabled:'已禁用'};
function renderAccounts() {
  const body = document.querySelector('#accounts-body');
  if (!XHS.state.accounts.length) {
    body.innerHTML = '<tr><td colspan="5" class="empty">暂无账号，请先创建</td></tr>';
    return;
  }
  body.innerHTML = XHS.state.accounts.map(account => `<tr><td><strong>${XHS.escapeHTML(account.display_name)}</strong><small>${XHS.escapeHTML(account.id)}</small></td><td>${XHS.escapeHTML(account.owner || '—')}</td><td><span class="badge badge-${XHS.escapeHTML(account.status)}">${statusLabel[account.status] || XHS.escapeHTML(account.status)}</span></td><td>${account.id === XHS.state.defaultAccountId ? '<span class="badge badge-default">默认</span>' : '—'}</td><td class="actions"><button data-action="qr" data-id="${XHS.escapeHTML(account.id)}">扫码登录</button><button data-action="default" data-id="${XHS.escapeHTML(account.id)}">设为默认</button><button data-action="reset" data-id="${XHS.escapeHTML(account.id)}" class="ghost">重置</button><button data-action="remove" data-id="${XHS.escapeHTML(account.id)}" class="danger ghost">删除</button></td></tr>`).join('');
}
async function refresh() { await XHS.loadAccounts(); renderAccounts(); }
async function openQR(id) {
  XHS.loading(true, '正在获取二维码…');
  try {
    const data = await XHS.api(`/api/web/accounts/${encodeURIComponent(id)}/login/qrcode`, {method:'POST', body:{}, account:false});
    if (data.is_logged_in) { XHS.toast('账号已登录'); await refresh(); return; }
    const image = data.image.startsWith('data:') ? data.image : `data:image/png;base64,${data.image}`;
    const dialog = document.querySelector('#qr-dialog');
    dialog.querySelector('img').src = image;
    dialog.querySelector('.qr-status').textContent = '请使用小红书扫码，正在等待登录…';
    dialog.showModal();
    clearInterval(loginTimer);
    loginTimer = setInterval(async () => {
      try {
        const status = await XHS.api(`/api/web/accounts/${encodeURIComponent(id)}/login/status`, {method:'POST', body:{}, account:false});
        if (status.is_logged_in) { clearInterval(loginTimer); dialog.close(); XHS.toast('登录成功'); await refresh(); }
      } catch (_) { /* 轮询期间保留二维码 */ }
    }, 3000);
  } catch (error) { XHS.toast(error.message, 'error'); }
  finally { XHS.loading(false); }
}
document.addEventListener('DOMContentLoaded', () => {
	document.querySelector('#refresh-accounts').addEventListener('click', refresh);
  document.querySelector('#create-account').addEventListener('submit', async event => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    XHS.loading(true, '正在创建账号…');
    try {
      await XHS.api('/api/web/accounts', {method:'POST', body:Object.fromEntries(form), account:false});
      event.currentTarget.reset(); XHS.toast('账号创建成功'); await refresh();
    } catch (error) { XHS.toast(error.message, 'error'); } finally { XHS.loading(false); }
  });
  document.querySelector('#accounts-body').addEventListener('click', async event => {
    const button = event.target.closest('button[data-action]'); if (!button) return;
    const {action, id} = button.dataset;
    if (action === 'qr') return openQR(id);
    if ((action === 'reset' || action === 'remove') && !confirm(action === 'remove' ? `确定删除账号 ${id}？此操作不可撤销。` : `确定重置账号 ${id} 的登录状态？`)) return;
    const request = action === 'default' ? {method:'PUT', path:`/api/web/accounts/${id}/default`} : action === 'reset' ? {method:'DELETE', path:`/api/web/accounts/${id}/login`} : {method:'DELETE', path:`/api/web/accounts/${id}`};
    try { await XHS.api(request.path, {method:request.method, account:false}); XHS.toast('操作成功'); await refresh(); } catch (error) { XHS.toast(error.message, 'error'); }
  });
  const dialog = document.querySelector('#qr-dialog');
  dialog.addEventListener('close', () => clearInterval(loginTimer));
  dialog.querySelector('button').addEventListener('click', () => dialog.close());
});
window.addEventListener('accountsready', renderAccounts);
