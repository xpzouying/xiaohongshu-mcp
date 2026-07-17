const discoverState = {
  recommendedLoaded: false,
  recommendedRequest: 0,
  recommendedController: null,
  searchRequest: 0,
  searchController: null,
  lastSearchInput: null,
  loadingRequests: new Set()
};

function beginLoading(label) {
  const request = Symbol('loading-request');
  discoverState.loadingRequests.add(request);
  XHS.loading(true, label);
  return request;
}

function endLoading(request) {
  if (!discoverState.loadingRequests.delete(request)) return;
  if (discoverState.loadingRequests.size === 0) XHS.loading(false);
}

const challengeCodes = new Set([
  'SEARCH_CHALLENGE', 'CAPTCHA_REQUIRED', 'VERIFICATION_REQUIRED', 'RISK_CONTROL'
]);

function feedDetailURL(feed) {
  return `/detail.html?feed_id=${encodeURIComponent(feed.id || '')}&xsec_token=${encodeURIComponent(feed.xsecToken || '')}`;
}

function userProfileURL(user, feed) {
  return `/profile.html?user_id=${encodeURIComponent(user.userId || '')}&xsec_token=${encodeURIComponent(feed.xsecToken || '')}`;
}

function renderFeeds(gridSelector, feeds, emptyMessage) {
  const grid = document.querySelector(gridSelector);
  if (!feeds.length) {
    grid.innerHTML = `<div class="empty card">${XHS.escapeHTML(emptyMessage)}</div>`;
    return;
  }
  grid.innerHTML = feeds.map(feed => {
    const note = feed.noteCard || {};
    const info = note.interactInfo || {};
    const user = note.user || {};
    const cover = note.cover || {};
    const rawCoverURL = cover.urlDefault || cover.url || '';
    // 小红书 CDN 返回 http，但页面 CSP 只允许 https；统一升级
    const coverURL = rawCoverURL ? rawCoverURL.replace(/^http:\/\//i, 'https://') : '';
    const author = user.nickname || user.nickName || '未知作者';
    const profileLink = user.userId
      ? `<a class="feed-author" href="${userProfileURL(user, feed)}">${XHS.escapeHTML(author)}</a>`
      : `<span class="feed-author">${XHS.escapeHTML(author)}</span>`;
    return `<article class="feed-card">` +
      `<a class="feed-cover" href="${feedDetailURL(feed)}">${coverURL ? `<img src="${XHS.escapeHTML(coverURL)}" alt="" loading="lazy">` : '<span class="cover-placeholder">无封面</span>'}</a>` +
      `<div class="feed-body"><h3><a href="${feedDetailURL(feed)}">${XHS.escapeHTML(note.displayTitle || '无标题')}</a></h3>${profileLink}` +
      `<div class="feed-meta"><span>♥ ${XHS.escapeHTML(info.likedCount || '0')}</span><span>☆ ${XHS.escapeHTML(info.collectedCount || '0')}</span><span>◌ ${XHS.escapeHTML(info.commentCount || '0')}</span></div></div></article>`;
  }).join('');
}

function feedList(data) {
  const feeds = Array.isArray(data) ? data : data.feeds;
  return Array.isArray(feeds) ? feeds : [];
}

function isSearchChallenge(error) {
  if (challengeCodes.has(String(error?.code || '').toUpperCase())) return true;
  const details = error?.details;
  if (!details || typeof details !== 'object') return false;
  if (details.challenge === true || details.captcha === true || details.verification_required === true) return true;
  return ['code', 'type', 'reason', 'stage', 'error']
    .map(key => details[key])
    .filter(value => typeof value === 'string')
    .some(value => /captcha|challenge|risk[_ -]?control|风控|安全验证/i.test(value));
}

function renderRequestError(gridSelector, error, retryAction, title = '加载失败') {
  const grid = document.querySelector(gridSelector);
  grid.innerHTML = `<div class="empty card request-error" role="alert">
    <h3>${XHS.escapeHTML(title)}</h3>
    <p>${XHS.escapeHTML(error?.message || '请求失败，请稍后重试')}</p>
    <button class="secondary" type="button" data-retry="${XHS.escapeHTML(retryAction)}">重试</button>
  </div>`;
}

function renderSearchChallenge() {
  document.querySelector('#search-grid').innerHTML = `<div class="empty card request-error" role="alert">
    <h3>搜索需要完成安全验证</h3>
    <p>服务返回了明确的安全验证信息。请先在小红书 App 内完成验证，再回来重试。</p>
    <div class="error-actions"><button class="secondary" type="button" data-retry="search">重试搜索</button><a class="primary" href="/accounts.html">去账号管理</a></div>
  </div>`;
}

function cancelRequests() {
  discoverState.recommendedRequest += 1;
  discoverState.searchRequest += 1;
  discoverState.recommendedController?.abort();
  discoverState.searchController?.abort();
  discoverState.recommendedController = null;
  discoverState.searchController = null;
  discoverState.loadingRequests.clear();
  XHS.loading(false);
}

function showTab(name) {
  document.querySelectorAll('.discover-tab').forEach(tab => {
    const active = tab.dataset.tab === name;
    tab.classList.toggle('active', active);
    tab.setAttribute('aria-selected', String(active));
  });
  document.querySelector('#recommended-panel').hidden = name !== 'recommended';
  document.querySelector('#search-panel').hidden = name !== 'search';
  if (name === 'recommended' && !discoverState.recommendedLoaded) loadRecommended();
}

async function loadRecommended() {
  if (!XHS.requireAccount()) return;
  discoverState.recommendedController?.abort();
  const controller = new AbortController();
  const request = ++discoverState.recommendedRequest;
  discoverState.recommendedController = controller;
  document.querySelector('#recommended-grid').innerHTML = '<div class="empty card">正在加载推荐内容…</div>';
  const loadingRequest = beginLoading('正在加载推荐内容…');
  try {
    const data = await XHS.callTool('list_feeds', {}, {signal: controller.signal});
    if (request !== discoverState.recommendedRequest) return;
    renderFeeds('#recommended-grid', feedList(data), '暂时没有推荐内容');
    discoverState.recommendedLoaded = true;
  } catch (error) {
    if (request !== discoverState.recommendedRequest || error?.code === 'REQUEST_ABORTED' || error?.name === 'AbortError') return;
    discoverState.recommendedLoaded = false;
    renderRequestError('#recommended-grid', error, 'recommended', '推荐内容加载失败');
  } finally {
    if (request === discoverState.recommendedRequest) discoverState.recommendedController = null;
    endLoading(loadingRequest);
  }
}

async function runSearch(input) {
  if (!XHS.requireAccount()) return;
  discoverState.searchController?.abort();
  const controller = new AbortController();
  const request = ++discoverState.searchRequest;
  discoverState.searchController = controller;
  discoverState.lastSearchInput = input;
  document.querySelector('#search-grid').innerHTML = '<div class="empty card">正在搜索…</div>';
  document.querySelector('#result-count').textContent = '正在搜索';
  const loadingRequest = beginLoading('正在搜索…');
  try {
    const data = await XHS.callTool('search_feeds', input, {signal: controller.signal});
    if (request !== discoverState.searchRequest) return;
    const feeds = feedList(data);
    renderFeeds('#search-grid', feeds, '没有找到相关笔记');
    document.querySelector('#result-count').textContent = `共 ${data.count ?? feeds.length} 条`;
  } catch (error) {
    if (request !== discoverState.searchRequest || error?.code === 'REQUEST_ABORTED' || error?.name === 'AbortError') return;
    if (isSearchChallenge(error)) {
      renderSearchChallenge();
      document.querySelector('#result-count').textContent = '搜索需要安全验证';
    } else {
      renderRequestError('#search-grid', error, 'search', '搜索失败');
      document.querySelector('#result-count').textContent = '搜索失败';
    }
  } finally {
    if (request === discoverState.searchRequest) discoverState.searchController = null;
    endLoading(loadingRequest);
  }
}

async function search(event) {
  event.preventDefault();
  const form = new FormData(event.currentTarget);
  const keyword = String(form.get('keyword') || '').trim();
  if (!keyword) {
    XHS.toast('请输入搜索关键词', 'warning');
    return;
  }
  const filters = {};
  ['sort_by', 'note_type', 'publish_time', 'search_scope', 'location'].forEach(key => { filters[key] = form.get(key); });
  return runSearch({keyword, filters});
}

document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('.discover-tab').forEach(tab => tab.addEventListener('click', () => showTab(tab.dataset.tab)));
  document.querySelector('#refresh-feeds').addEventListener('click', () => {
    discoverState.recommendedLoaded = false;
    loadRecommended();
  });
  document.querySelector('#search-form').addEventListener('submit', search);
  document.addEventListener('click', event => {
    const action = event.target.closest?.('[data-retry]')?.dataset.retry;
    if (action === 'recommended') loadRecommended();
    if (action === 'search' && discoverState.lastSearchInput) runSearch(discoverState.lastSearchInput);
  });
});
window.addEventListener('accountsready', loadRecommended);
window.addEventListener('accountchange', () => {
  cancelRequests();
  discoverState.recommendedLoaded = false;
  discoverState.lastSearchInput = null;
  document.querySelector('#search-grid').innerHTML = '<div class="empty card">输入关键词开始搜索</div>';
  document.querySelector('#result-count').textContent = '等待搜索';
  if (!document.querySelector('#recommended-panel').hidden) loadRecommended();
});
