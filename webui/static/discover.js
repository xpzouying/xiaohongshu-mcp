const discoverState = {recommendedLoaded: false};

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
  XHS.loading(true, '正在加载推荐内容…');
  try {
    const data = await XHS.callTool('list_feeds');
    renderFeeds('#recommended-grid', feedList(data), '暂时没有推荐内容');
    discoverState.recommendedLoaded = true;
  } catch (error) {
    XHS.toast(error.message, 'error');
  } finally {
    XHS.loading(false);
  }
}

async function search(event) {
  event.preventDefault();
  if (!XHS.requireAccount()) return;
  const form = new FormData(event.currentTarget);
  const keyword = String(form.get('keyword') || '').trim();
  if (!keyword) {
    XHS.toast('请输入搜索关键词', 'warning');
    return;
  }
  const filters = {};
  ['sort_by', 'note_type', 'publish_time', 'search_scope', 'location'].forEach(key => { filters[key] = form.get(key); });
  XHS.loading(true, '正在搜索…');
  try {
    const data = await XHS.callTool('search_feeds', {keyword, filters});
    const feeds = feedList(data);
    renderFeeds('#search-grid', feeds, '没有找到相关笔记');
    document.querySelector('#result-count').textContent = `共 ${data.count ?? feeds.length} 条`;
  } catch (error) {
    const isChallenge = error.code === 'SEARCH_TIMEOUT' || error.status === 504 || /captcha|verify|风控/i.test(error.message);
    if (isChallenge) {
      const grid = document.querySelector('#search-grid');
      grid.innerHTML = `<div class="empty card">
        <p>搜索当前被小红书安全验证（风控）拦截。</p>
        <p>这<strong>不是页面或系统故障</strong>。请尝试：</p>
        <ol style="text-align:left;max-width:28em;margin:0 auto">
          <li>打开小红书 App → 我的 → 设置，检查是否有安全提示；</li>
          <li>在 App 内搜索一次任意关键词完成验证；</li>
          <li>稍等几分钟后在此页面重试。</li>
        </ol>
      </div>`;
      document.querySelector('#result-count').textContent = '搜索被风控拦截';
    } else {
      XHS.toast(error.message, 'error');
    }
  } finally {
    XHS.loading(false);
  }
}

document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('.discover-tab').forEach(tab => tab.addEventListener('click', () => showTab(tab.dataset.tab)));
  document.querySelector('#refresh-feeds').addEventListener('click', () => {
    discoverState.recommendedLoaded = false;
    loadRecommended();
  });
  document.querySelector('#search-form').addEventListener('submit', search);
});
window.addEventListener('accountsready', loadRecommended);
window.addEventListener('accountchange', () => {
  discoverState.recommendedLoaded = false;
  if (!document.querySelector('#recommended-panel').hidden) loadRecommended();
});
