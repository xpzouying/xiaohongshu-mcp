const profileParams = new URLSearchParams(location.search);
const profileState = {
  userId: profileParams.get('user_id') || '',
  token: profileParams.get('xsec_token') || '',
  feeds: [],
  visibleFeedCount: 0,
  requestId: 0
};
const PROFILE_FEED_BATCH_SIZE = 12;

function safeImageURL(value) {
  if (!value) return '';
  try {
    const url = new URL(String(value), location.origin);
    if (url.protocol === 'http:') url.protocol = 'https:';
    if (url.protocol === 'https:' || (url.protocol === 'data:' && url.pathname.startsWith('image/'))) return url.href;
  } catch (_) {
    return '';
  }
  return '';
}

function interactionValue(interactions, type, name) {
  const item = interactions.find(value => value.type === type || value.name === name);
  return item?.count ?? '0';
}

function renderProfile(payload) {
  const data = payload.data || payload;
  const basic = data.userBasicInfo || data.basicInfo || {};
  const interactions = Array.isArray(data.interactions) ? data.interactions : [];
  const feeds = Array.isArray(data.feeds) ? data.feeds : [];
  const avatar = safeImageURL(basic.imageb || basic.images || basic.avatar);
  const stats = [
    ['关注', interactionValue(interactions, 'follows', '关注')],
    ['粉丝', interactionValue(interactions, 'fans', '粉丝')],
    ['获赞与收藏', interactionValue(interactions, 'interaction', '获赞与收藏')]
  ];

  document.querySelector('#profile-content').innerHTML = `<article class="card profile-card">${avatar ? `<img class="profile-avatar" src="${XHS.escapeHTML(avatar)}" alt="用户头像">` : '<div class="profile-avatar" aria-hidden="true"></div>'}<div class="profile-copy"><h1>${XHS.escapeHTML(basic.nickname || '未知用户')}</h1><p class="profile-id">小红书号：${XHS.escapeHTML(basic.redId || '未公开')}</p><p>${XHS.escapeHTML(basic.desc || '暂无简介')}</p><p>${XHS.escapeHTML(basic.ipLocation || '')}</p></div><ul class="profile-stats">${stats.map(([label, count]) => `<li class="profile-stat"><strong>${XHS.escapeHTML(count)}</strong><span>${XHS.escapeHTML(label)}</span></li>`).join('')}</ul></article>`;
  renderProfileFeeds(feeds);
}

function renderProfileFeeds(feeds) {
  profileState.feeds = feeds;
  profileState.visibleFeedCount = Math.min(PROFILE_FEED_BATCH_SIZE, feeds.length);
  renderVisibleProfileFeeds();
}

function renderVisibleProfileFeeds() {
  const grid = document.querySelector('#profile-feeds');
  const more = document.querySelector('#profile-more');
  const total = profileState.feeds.length;
  const visible = profileState.visibleFeedCount;
  document.querySelector('#note-count').textContent = visible < total ? `已显示 ${visible} / 共 ${total} 篇` : `共 ${total} 篇`;
  if (!total) {
    grid.innerHTML = '<div class="empty card">该用户暂未发布笔记</div>';
    more.hidden = true;
    return;
  }
  grid.innerHTML = profileState.feeds.slice(0, visible).map(feed => {
    const note = feed.noteCard || {};
    const info = note.interactInfo || {};
    const cover = note.cover || {};
    const coverURL = safeImageURL(cover.urlDefault || cover.url || cover.urlPre);
    const detailURL = `/detail.html?feed_id=${encodeURIComponent(String(feed.id || ''))}&xsec_token=${encodeURIComponent(String(feed.xsecToken || ''))}`;
    return `<a class="feed-card" href="${detailURL}">${coverURL ? `<img src="${XHS.escapeHTML(coverURL)}" alt="" loading="lazy">` : '<div class="cover-placeholder">无封面</div>'}<div class="feed-body"><h3>${XHS.escapeHTML(note.displayTitle || '无标题')}</h3><div class="feed-meta"><span>♥ ${XHS.escapeHTML(info.likedCount || '0')}</span><span>☆ ${XHS.escapeHTML(info.collectedCount || '0')}</span><span>◌ ${XHS.escapeHTML(info.commentCount || '0')}</span></div></div></a>`;
  }).join('');
  more.hidden = visible >= total;
  more.textContent = more.hidden ? '' : `加载更多（再加载 ${Math.min(PROFILE_FEED_BATCH_SIZE, total - visible)} 篇）`;
}

function loadMoreProfileFeeds() {
  profileState.visibleFeedCount = Math.min(profileState.visibleFeedCount + PROFILE_FEED_BATCH_SIZE, profileState.feeds.length);
  renderVisibleProfileFeeds();
}

function resetProfileView() {
  profileState.feeds = [];
  profileState.visibleFeedCount = 0;
  document.querySelector('#profile-content').innerHTML = '<div class="empty card">正在加载用户主页…</div>';
  document.querySelector('#profile-feeds').innerHTML = '<div class="empty card">正在加载笔记…</div>';
  document.querySelector('#note-count').textContent = '';
  document.querySelector('#profile-more').hidden = true;
}

async function loadProfile() {
  const requestId = ++profileState.requestId;
  resetProfileView();
  if (!profileState.userId || !profileState.token) {
    document.querySelector('#profile-content').innerHTML = '<div class="empty card">请在地址中提供 user_id 与 xsec_token，或从用户入口进入主页。</div>';
    document.querySelector('#profile-feeds').innerHTML = '<div class="empty card">暂无可展示的笔记</div>';
    return;
  }
  if (!XHS.requireAccount()) return;
  XHS.loading(true, '正在加载用户主页…');
  try {
    const data = await XHS.callTool('user_profile', {user_id: profileState.userId, xsec_token: profileState.token});
    if (requestId !== profileState.requestId) return;
    renderProfile(data);
  } catch (error) {
    if (requestId !== profileState.requestId) return;
    XHS.toast(error.message, 'error');
    document.querySelector('#profile-content').innerHTML = `<div class="empty card"><p>${XHS.escapeHTML(error.message)}</p><button type="button" data-action="retry-profile">重试</button></div>`;
    document.querySelector('#profile-feeds').innerHTML = '<div class="empty card">笔记加载失败，请重试</div>';
  } finally {
    if (requestId === profileState.requestId) XHS.loading(false);
  }
}

function retryProfile(event) {
  event?.preventDefault();
  return loadProfile();
}

document.querySelector('#profile-more').addEventListener('click', loadMoreProfileFeeds);
document.querySelector('#profile-content').addEventListener('click', event => {
  if (event.target.closest?.('[data-action="retry-profile"]')) retryProfile(event);
});

window.addEventListener('accountsready', loadProfile);
window.addEventListener('accountchange', loadProfile);