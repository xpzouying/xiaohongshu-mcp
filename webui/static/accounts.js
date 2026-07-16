let loginTimer;
let pendingAccountId = null;
const statusLabel = {active:'已登录', needs_login:'需登录', paused:'已暂停', risk_hold:'风控冻结', disabled:'已禁用'};

function renderAccounts() {
  const body = document.querySelector('#accounts-body');
  if (!XHS.state.accounts.length) {
    body.innerHTML = '<tr><td colspan="4" class="empty">暂无账号，点击上方按钮扫码添加</td></tr>';
    return;
  }
  body.innerHTML = XHS.state.accounts.map(account => `<tr>
    <td><strong>${XHS.escapeHTML(account.display_name)}</strong><small>${XHS.escapeHTML(account.id)}</small></td>
    <td><span class="badge badge-${XHS.escapeHTML(account.status)}">${statusLabel[account.status] || XHS.escapeHTML(account.status)}</span></td>
    <td>${account.id === XHS.state.defaultAccountId ? '<span class="badge badge-default">默认</span>' : '—'}</td>
    <td class="actions">
      <button data-action="qr" data-id="${XHS.escapeHTML(account.id)}">扫码登录</button>
      <button data-action="default" data-id="${XHS.escapeHTML(account.id)}">设为默认</button>
      <button data-action="reset" data-id="${XHS.escapeHTML(account.id)}" class="ghost">重置</button>
      <button data-action="remove" data-id="${XHS.escapeHTML(account.id)}" class="danger ghost">删除</button>
    </td></tr>`).join('');
}

async function refresh() {
  await XHS.loadAccounts();
  renderAccounts();
}

// 快速添加：后端自动创建账号槽位并返回二维码
async function quickAdd() {
  XHS.loading(true, '正在生成二维码…');
  try {
    const result = await XHS.api('/api/web/accounts/quick_add', {method:'POST', body:{}, account:false});
    const account = result.account || {};
    const qrcode = result.qrcode || {};
    pendingAccountId = account.id;
    showQRDialog(qrcode.image || qrcode.qr_code || '', account.id);
  } catch (error) {
    XHS.toast(error.message || '生成二维码失败', 'error');
  } finally {
    XHS.loading(false);
  }
}

// 对已有账号获取二维码
async function openQR(id) {
  XHS.loading(true, '正在获取二维码…');
  try {
    const data = await XHS.api(`/api/web/accounts/${encodeURIComponent(id)}/login/qrcode`, {method:'POST', body:{}, account:false});
    if (data.is_logged_in) { XHS.toast('账号已登录'); await refresh(); return; }
    pendingAccountId = id;
    showQRDialog(data.image || '', id);
  } catch (error) {
    XHS.toast(error.message || '获取二维码失败', 'error');
  } finally {
    XHS.loading(false);
  }
}

function showQRDialog(imageBase64, accountId) {
  const image = imageBase64.startsWith('data:') ? imageBase64 : `data:image/png;base64,${imageBase64}`;
  const dialog = document.querySelector('#qr-dialog');
  dialog.querySelector('img').src = image;
  dialog.querySelector('.qr-status').textContent = '请使用小红书 App 扫码登录…';
  dialog.showModal();
  clearInterval(loginTimer);
  // 每 3 秒轮询登录状态
  loginTimer = setInterval(async () => {
    try {
      const status = await XHS.api(`/api/web/accounts/${encodeURIComponent(accountId)}/login/status`, {method:'POST', body:{}, account:false});
      if (status.is_logged_in) {
        clearInterval(loginTimer);
        dialog.close();
        XHS.toast('登录成功，正在同步账号信息…');
        // 自动同步昵称
        try {
          await XHS.api(`/api/web/accounts/${encodeURIComponent(accountId)}/sync_profile`, {method:'POST', body:{}, account:false});
        } catch (_) { /* 同步失败不阻塞 */ }
        XHS.toast('登录成功');
        await refresh();
      }
    } catch (_) { /* 轮询期间保留二维码 */ }
  }, 3000);
}

document.addEventListener('DOMContentLoaded', () => {
  document.querySelector('#refresh-accounts').addEventListener('click', refresh);
  document.querySelector('#quick-add-btn').addEventListener('click', quickAdd);
  document.querySelector('#accounts-body').addEventListener('click', async event => {
    const button = event.target.closest('button[data-action]');
    if (!button) return;
    const {action, id} = button.dataset;
    if (action === 'qr') return openQR(id);
    if ((action === 'reset' || action === 'remove') && !confirm(action === 'remove' ? `确定删除账号 ${id}？此操作不可撤销。` : `确定重置账号 ${id} 的登录状态？`)) return;
    const request = action === 'default'
      ? {method:'PUT', path:`/api/web/accounts/${id}/default`}
      : action === 'reset'
        ? {method:'DELETE', path:`/api/web/accounts/${id}/login`}
        : {method:'DELETE', path:`/api/web/accounts/${id}`};
    try {
      await XHS.api(request.path, {method:request.method, account:false});
      XHS.toast('操作成功');
      await refresh();
    } catch (error) {
      XHS.toast(error.message || '操作失败', 'error');
    }
  });
  const dialog = document.querySelector('#qr-dialog');
  dialog.addEventListener('close', () => clearInterval(loginTimer));
  dialog.querySelector('button').addEventListener('click', () => dialog.close());
});
window.addEventListener('accountsready', renderAccounts);
