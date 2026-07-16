const XHS = (() => {
  const state = { accounts: [], defaultAccountId: '', selectedAccountId: localStorage.getItem('selected_account_id') || '' };
  const errorMessages = {
    ACCOUNT_LOGIN_REQUIRED: '当前账号需要扫码登录', ACCOUNT_BUSY: '账号正在执行其他操作，请稍后重试',
    ACCOUNT_RISK_HOLD: '账号处于风控冻结状态', UPSTREAM_UNAVAILABLE: '小红书服务暂时不可用'
  };

  function escapeHTML(value = '') {
    return String(value).replace(/[&<>'"]/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[char]));
  }
  function toast(message, type = 'success') {
    const region = document.querySelector('#toast-region');
    const item = document.createElement('div');
    item.className = `toast toast-${type}`;
    item.textContent = message;
    region.append(item);
    setTimeout(() => item.remove(), 3500);
  }
  function loading(show, label = '正在处理…') {
    const overlay = document.querySelector('#loading-overlay');
    overlay.querySelector('span').textContent = label;
    overlay.hidden = !show;
  }
  async function api(path, options = {}) {
    const method = (options.method || 'GET').toUpperCase();
    const accountId = options.account === false ? '' : state.selectedAccountId;
    let url = path;
    const init = {...options, method};
    delete init.account;
    if (accountId && method === 'GET') {
      const separator = url.includes('?') ? '&' : '?';
      url += `${separator}account_id=${encodeURIComponent(accountId)}`;
    } else if (accountId && init.body) {
      const body = typeof init.body === 'string' ? JSON.parse(init.body) : init.body;
      init.body = JSON.stringify({...body, account_id: accountId});
    }
    if (init.body) init.headers = {'Content-Type': 'application/json', ...(init.headers || {})};
    const response = await fetch(url, init);
    let payload;
    try { payload = await response.json(); } catch (_) { payload = {}; }
    if (!response.ok || payload.success === false || payload.error) {
      const error = new Error(errorMessages[payload.code] || payload.error || `请求失败 (${response.status})`);
      error.code = payload.code;
      throw error;
    }
    return Object.prototype.hasOwnProperty.call(payload, 'data') ? payload.data : payload;
  }
  function currentAccount() { return state.accounts.find(item => item.id === state.selectedAccountId); }
  function requireAccount() {
    if (state.selectedAccountId) return true;
    toast('请先选择或创建账号', 'warning');
    return false;
  }
  async function loadAccounts() {
    try {
      const data = await api('/api/web/accounts', {account: false});
      state.accounts = data.accounts || [];
      state.defaultAccountId = data.default_account_id || '';
      const selectable = account => !['disabled', 'risk_hold'].includes(account.status);
      const saved = state.accounts.find(account => account.id === state.selectedAccountId && selectable(account));
      const fallback = state.accounts.find(account => account.id === state.defaultAccountId && selectable(account)) || state.accounts.find(selectable);
      state.selectedAccountId = saved?.id || fallback?.id || '';
      if (state.selectedAccountId) localStorage.setItem('selected_account_id', state.selectedAccountId);
      else localStorage.removeItem('selected_account_id');
      renderAccountSelector();
    } catch (error) {
      renderAccountSelector();
      toast(error.message, 'error');
    }
  }
  function renderAccountSelector() {
    const select = document.querySelector('#global-account');
    if (!select) return;
    select.innerHTML = '<option value="">选择账号</option>' + state.accounts.map(account => {
      const disabled = ['disabled', 'risk_hold'].includes(account.status) ? ' disabled' : '';
      return `<option value="${escapeHTML(account.id)}"${disabled}>${escapeHTML(account.display_name)} (${escapeHTML(account.id)})</option>`;
    }).join('');
    select.value = state.selectedAccountId;
  }
  function initShell() {
    const page = document.body.dataset.page;
    const links = [['dashboard','/','概览'],['accounts','/accounts.html','账号'],['search','/search.html','搜索'],['publish','/publish.html','发布'],['detail','/detail.html','详情']];
    document.querySelector('#app-header').innerHTML = `<a class="brand" href="/" aria-label="小红书 MCP 首页"><span>小</span>红书 MCP</a><nav aria-label="主导航">${links.map(([id, href, label]) => `<a href="${href}"${id === page ? ' aria-current="page"' : ''}>${label}</a>`).join('')}</nav><label class="account-picker"><span>当前账号</span><select id="global-account" aria-label="当前账号"><option>加载中…</option></select></label>`;
    document.body.insertAdjacentHTML('beforeend', '<div id="toast-region" class="toast-region" role="status" aria-live="polite"></div><div id="loading-overlay" class="loading-overlay" hidden><div class="spinner" aria-hidden="true"></div><span>正在处理…</span></div>');
    document.querySelector('#global-account').addEventListener('change', event => {
      state.selectedAccountId = event.target.value;
      if (state.selectedAccountId) localStorage.setItem('selected_account_id', state.selectedAccountId);
      else localStorage.removeItem('selected_account_id');
      window.dispatchEvent(new CustomEvent('accountchange', {detail: state.selectedAccountId}));
    });
    loadAccounts().then(() => window.dispatchEvent(new CustomEvent('accountsready')));
  }
  document.addEventListener('DOMContentLoaded', initShell);
  return {state, api, toast, loading, escapeHTML, currentAccount, requireAccount, loadAccounts};
})();
