let loginTimer;
let pendingAccountId = null;
const statusLabel = {active:'已登录', needs_login:'需登录', paused:'已暂停', risk_hold:'风控冻结', disabled:'已禁用'};
const reservedAccountIds = new Set(['accounts', 'system', 'root', 'null', 'unknown']);

function validateAccountId(value) {
  if (!/^[a-z][a-z0-9_]{2,31}$/.test(value) || reservedAccountIds.has(value)) {
    throw new Error('账号 ID 须为 3～32 位小写字母、数字或下划线，且不能使用保留名称');
  }
}

async function createAccount(event) {
  event.preventDefault();
  const form = event.currentTarget, submit = form.querySelector('[type="submit"]');
  const values = new FormData(form);
  const body = {id:String(values.get('account_id') || '').trim(), display_name:String(values.get('display_name') || '').trim(), owner:String(values.get('owner') || '').trim(), purpose:String(values.get('purpose') || '').trim()};
  try {
    validateAccountId(body.id);
    if (!body.display_name) throw new Error('显示名称不能为空');
    submit.disabled = true; submit.textContent = '创建中…';
    await XHS.callTool('create_account', {account_id:body.id, display_name:body.display_name, owner:body.owner, purpose:body.purpose});
    form.reset(); document.querySelector('#advanced-create').open = false;
    XHS.toast('账号创建成功，可继续扫码登录'); await refresh();
  } catch (error) { XHS.toast(error.message || '创建账号失败', 'error'); }
  finally { submit.disabled = false; submit.textContent = '创建账号'; }
}

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
      <button data-action="status" data-id="${XHS.escapeHTML(account.id)}">检查状态</button>
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
    const data = await XHS.callTool('get_login_qrcode', {account_id:id});
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
      const status = await XHS.callTool('check_login_status', {account_id:accountId});
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

async function checkLoginStatus(id, button) {
  button.disabled = true;
  const oldLabel = button.textContent; button.textContent = '检查中…';
  try {
    const status = await XHS.callTool('check_login_status', {account_id:id});
    const identity = status.identity?.nickname || status.identity?.display_name || '';
    XHS.toast(status.is_logged_in ? `账号已登录${identity ? `：${identity}` : ''}` : '账号尚未登录', status.is_logged_in ? 'success' : 'warning');
    await refresh();
  } catch (error) { XHS.toast(error.message || '检查登录状态失败', 'error'); }
  finally { button.disabled = false; button.textContent = oldLabel; }
}

document.addEventListener('DOMContentLoaded', () => {
  document.querySelector('#refresh-accounts').addEventListener('click', refresh);
  document.querySelector('#quick-add-btn').addEventListener('click', quickAdd);
  document.querySelector('#create-account-form').addEventListener('submit', createAccount);
  document.querySelector('#cancel-create').addEventListener('click', () => { document.querySelector('#advanced-create').open = false; });
  document.querySelector('#accounts-body').addEventListener('click', async event => {
    const button = event.target.closest('button[data-action]');
    if (!button) return;
    const {action, id} = button.dataset;
    if (action === 'qr') return openQR(id);
    if (action === 'status') return checkLoginStatus(id, button);
    if ((action === 'reset' || action === 'remove') && !confirm(action === 'remove' ? `确定删除账号 ${id}？此操作不可撤销。` : `确定重置账号 ${id} 的登录状态？`)) return;
    const tool = action === 'default' ? 'set_default_account' : action === 'reset' ? 'reset_login' : 'remove_account';
    try {
      await XHS.callTool(tool, {account_id:id});
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
