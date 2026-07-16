function renderFeeds(feeds) {
  const grid = document.querySelector('#feed-grid');
  if (!feeds.length) { grid.innerHTML = '<div class="empty card">没有找到相关笔记</div>'; return; }
  grid.innerHTML = feeds.map(feed => {
    const note = feed.noteCard || {}; const info = note.interactInfo || {}; const user = note.user || {}; const cover = note.cover || {};
    const url = `/detail.html?feed_id=${encodeURIComponent(feed.id)}&xsec_token=${encodeURIComponent(feed.xsecToken || '')}`;
    return `<a class="feed-card" href="${url}">${cover.urlDefault || cover.url ? `<img src="${XHS.escapeHTML(cover.urlDefault || cover.url)}" alt="" loading="lazy">` : '<div class="cover-placeholder">无封面</div>'}<div class="feed-body"><h3>${XHS.escapeHTML(note.displayTitle || '无标题')}</h3><p>${XHS.escapeHTML(user.nickname || user.nickName || '未知作者')}</p><div class="feed-meta"><span>♥ ${XHS.escapeHTML(info.likedCount || '0')}</span><span>☆ ${XHS.escapeHTML(info.collectedCount || '0')}</span><span>◌ ${XHS.escapeHTML(info.commentCount || '0')}</span></div></div></a>`;
  }).join('');
}
async function search(event) {
  event?.preventDefault(); if (!XHS.requireAccount()) return;
  const form = new FormData(document.querySelector('#search-form'));
  const keyword = String(form.get('keyword') || '').trim(); if (!keyword) { XHS.toast('请输入搜索关键词', 'warning'); return; }
  const filters = {}; ['sort_by','note_type','publish_time','search_scope','location'].forEach(key => filters[key] = form.get(key));
  XHS.loading(true, '正在搜索…');
  try { const data = await XHS.api('/api/web/feeds/search', {method:'POST', body:{keyword, filters}}); renderFeeds(data.feeds || []); document.querySelector('#result-count').textContent = `共 ${data.count ?? data.feeds?.length ?? 0} 条`; }
  catch (error) { XHS.toast(error.message, 'error'); } finally { XHS.loading(false); }
}
document.addEventListener('DOMContentLoaded', () => document.querySelector('#search-form').addEventListener('submit', search));
