const XHS = (() => {
  const state = { accounts: [], defaultAccountId: '', selectedAccountId: localStorage.getItem('selected_account_id') || '' };
  const errorMessages = {
    ACCOUNT_LOGIN_REQUIRED: '当前账号需要扫码登录', ACCOUNT_BUSY: '账号正在执行其他操作，请稍后重试',
    ACCOUNT_RISK_HOLD: '账号处于风控冻结状态', UPSTREAM_UNAVAILABLE: '小红书服务暂时不可用'
  };

  function normalizeError(error, fallback = {}) {
    if (error?.name === 'AbortError') {
      return Object.assign(new Error('请求已取消'), {name: 'AbortError', code: 'REQUEST_ABORTED', status: 0});
    }
    const normalized = error instanceof Error ? error : new Error(fallback.message || '请求失败');
    normalized.code = fallback.code || normalized.code || 'REQUEST_FAILED';
    normalized.status = fallback.status ?? normalized.status ?? 0;
    normalized.details = fallback.details ?? normalized.details;
    return normalized;
  }

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
    let response;
    try {
      response = await fetch(url, init);
    } catch (error) {
      throw normalizeError(error, {code: error?.name === 'AbortError' ? 'REQUEST_ABORTED' : 'NETWORK_ERROR'});
    }
    let payload;
    try { payload = await response.json(); } catch (_) {
      if (response.status === 204) return null;
      throw normalizeError(null, {message: '服务端返回了无法解析的响应', code: 'INVALID_RESPONSE', status: response.status});
    }
    if (!response.ok || payload.success === false || payload.error) {
      throw normalizeError(null, {
        message: errorMessages[payload.code] || payload.error || `请求失败 (${response.status})`,
        code: payload.code || 'REQUEST_FAILED', status: response.status, details: payload.details
      });
    }
    return Object.prototype.hasOwnProperty.call(payload, 'data') ? payload.data : payload;
  }
  function callTool(toolName, input = {}, options = {}) {
    if (!globalThis.XHSMCP?.callTool) throw new Error('MCP 调用层尚未初始化');
    return globalThis.XHSMCP.callTool(toolName, input, {...options, api});
  }
  function currentAccount() { return state.accounts.find(item => item.id === state.selectedAccountId); }
  function requireAccount() {
    if (state.selectedAccountId) return true;
    toast('请先选择或创建账号', 'warning');
    return false;
  }
  async function loadAccounts() {
    try {
      const data = await callTool('list_accounts');
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
    const links = [['dashboard','/','概览'],['accounts','/accounts.html','账号'],['discover','/discover.html','发现'],['publish','/publish.html','发布'],['profile','/profile.html','用户主页']];
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
  return {state, api, callTool, normalizeError, toast, loading, escapeHTML, currentAccount, requireAccount, loadAccounts};
})();
