const profileParams = new URLSearchParams(location.search);
const profileState = {
  userId: profileParams.get('user_id') || '',
  token: profileParams.get('xsec_token') || ''
};

function safeImageURL(value) {
  if (!value) return '';
  try {
    const url = new URL(String(value), location.origin);
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
  const grid = document.querySelector('#profile-feeds');
  document.querySelector('#note-count').textContent = `共 ${feeds.length} 篇`;
  if (!feeds.length) {
    grid.innerHTML = '<div class="empty card">该用户暂未发布笔记</div>';
    return;
  }
  grid.innerHTML = feeds.map(feed => {
    const note = feed.noteCard || {};
    const info = note.interactInfo || {};
    const cover = note.cover || {};
    const coverURL = safeImageURL(cover.urlDefault || cover.url || cover.urlPre);
    const detailURL = `/detail.html?feed_id=${encodeURIComponent(String(feed.id || ''))}&xsec_token=${encodeURIComponent(String(feed.xsecToken || ''))}`;
    return `<a class="feed-card" href="${detailURL}">${coverURL ? `<img src="${XHS.escapeHTML(coverURL)}" alt="" loading="lazy">` : '<div class="cover-placeholder">无封面</div>'}<div class="feed-body"><h3>${XHS.escapeHTML(note.displayTitle || '无标题')}</h3><div class="feed-meta"><span>♥ ${XHS.escapeHTML(info.likedCount || '0')}</span><span>☆ ${XHS.escapeHTML(info.collectedCount || '0')}</span><span>◌ ${XHS.escapeHTML(info.commentCount || '0')}</span></div></div></a>`;
  }).join('');
}

async function loadProfile() {
  if (!profileState.userId || !profileState.token) {
    document.querySelector('#profile-content').innerHTML = '<div class="empty card">请在地址中提供 user_id 与 xsec_token，或从用户入口进入主页。</div>';
    document.querySelector('#profile-feeds').innerHTML = '<div class="empty card">暂无可展示的笔记</div>';
    return;
  }
  if (!XHS.requireAccount()) return;
  XHS.loading(true, '正在加载用户主页…');
  try {
    const data = await XHS.callTool('user_profile', {user_id: profileState.userId, xsec_token: profileState.token});
    renderProfile(data);
  } catch (error) {
    XHS.toast(error.message, 'error');
    document.querySelector('#profile-content').innerHTML = `<div class="empty card">${XHS.escapeHTML(error.message)}</div>`;
  } finally {
    XHS.loading(false);
  }
}

window.addEventListener('accountsready', loadProfile);
window.addEventListener('accountchange', loadProfile);