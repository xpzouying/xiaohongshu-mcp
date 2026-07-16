const params = new URLSearchParams(location.search);
const detailState = {feedId: params.get('feed_id') || '', token: params.get('xsec_token') || '', note: null};
function imageURL(image) { return image.urlDefault || image.urlPre || image.url || ''; }
function renderComment(comment, child = false) {
  const user = comment.userInfo || {};
  return `<article class="comment${child ? ' sub-comment' : ''}"><div><strong>${XHS.escapeHTML(user.nickname || '用户')}</strong><small>${XHS.escapeHTML(comment.ipLocation || '')}</small></div><p>${XHS.escapeHTML(comment.content || '')}</p><button class="link-button reply-button" data-comment-id="${XHS.escapeHTML(comment.id || '')}" data-user-id="${XHS.escapeHTML(user.userId || '')}">回复</button>${(comment.subComments || []).map(item => renderComment(item, true)).join('')}</article>`;
}
function renderDetail(payload) {
  const data = payload.data || payload; const note = data.note || {}; detailState.note = note;
  const user = note.user || {}, info = note.interactInfo || {};
  document.querySelector('#detail-content').innerHTML = `<div class="author">${user.avatar ? `<img src="${XHS.escapeHTML(user.avatar)}" alt="">` : ''}<div><strong>${XHS.escapeHTML(user.nickname || '未知作者')}</strong><small>${XHS.escapeHTML(note.ipLocation || '')}</small></div></div><h1>${XHS.escapeHTML(note.title || '无标题')}</h1><p class="note-desc">${XHS.escapeHTML(note.desc || '')}</p><div class="image-gallery">${(note.imageList || []).map(image => `<img src="${XHS.escapeHTML(imageURL(image))}" alt="笔记图片" loading="lazy">`).join('')}</div><div class="interaction-bar"><button id="like-button" class="${info.liked ? 'active' : ''}">♥ ${XHS.escapeHTML(info.likedCount || '0')}</button><button id="favorite-button" class="${info.collected ? 'active' : ''}">☆ ${XHS.escapeHTML(info.collectedCount || '0')}</button><span>评论 ${XHS.escapeHTML(info.commentCount || '0')}</span><span>分享 ${XHS.escapeHTML(info.sharedCount || '0')}</span></div>`;
  document.querySelector('#comments').innerHTML = (data.comments?.list || []).map(item => renderComment(item)).join('') || '<p class="empty">暂无评论</p>';
  document.querySelector('#like-button').addEventListener('click', () => toggleAction('like', info.liked));
  document.querySelector('#favorite-button').addEventListener('click', () => toggleAction('favorite', info.collected));
}
async function loadDetail() {
  if (!detailState.feedId || !detailState.token) { document.querySelector('#detail-content').innerHTML = '<div class="empty">请从搜索结果进入，或在地址中提供 feed_id 与 xsec_token。</div>'; return; }
  if (!XHS.requireAccount()) return; XHS.loading(true, '正在加载笔记…');
  try { const data = await XHS.api('/api/web/feeds/detail', {method:'POST', body:{feed_id:detailState.feedId, xsec_token:detailState.token, load_all_comments:false}}); renderDetail(data); }
  catch (error) { XHS.toast(error.message, 'error'); } finally { XHS.loading(false); }
}
async function toggleAction(action, active) {
  try { await XHS.api(`/api/web/feeds/${action}`, {method:'POST', body:{feed_id:detailState.feedId, xsec_token:detailState.token, [action === 'like' ? 'unlike' : 'unfavorite']:active}}); XHS.toast(active ? '已取消' : '操作成功'); await loadDetail(); }
  catch (error) { XHS.toast(error.message, 'error'); }
}
document.addEventListener('DOMContentLoaded', () => {
  document.querySelector('#comment-form').addEventListener('submit', async event => {
    event.preventDefault(); const content = new FormData(event.currentTarget).get('content').trim(); if (!content) return;
    try { await XHS.api('/api/web/feeds/comment', {method:'POST', body:{feed_id:detailState.feedId, xsec_token:detailState.token, content}}); event.currentTarget.reset(); XHS.toast('评论发表成功'); await loadDetail(); } catch (error) { XHS.toast(error.message, 'error'); }
  });
  document.querySelector('#comments').addEventListener('click', event => {
    const button = event.target.closest('.reply-button'); if (!button) return;
    document.querySelector('#reply-comment-id').value = button.dataset.commentId;
    document.querySelector('#reply-user-id').value = button.dataset.userId;
    document.querySelector('#reply-dialog').showModal();
  });
  document.querySelector('#reply-form').addEventListener('submit', async event => {
    event.preventDefault(); const form = new FormData(event.currentTarget);
    try { await XHS.api('/api/web/feeds/comment/reply', {method:'POST', body:{feed_id:detailState.feedId,xsec_token:detailState.token,comment_id:form.get('comment_id'),user_id:form.get('user_id'),content:form.get('content')}}); document.querySelector('#reply-dialog').close(); XHS.toast('回复成功'); await loadDetail(); } catch (error) { XHS.toast(error.message, 'error'); }
  });
  document.querySelector('#cancel-reply').addEventListener('click', () => document.querySelector('#reply-dialog').close());
});
window.addEventListener('accountsready', loadDetail);
window.addEventListener('accountchange', loadDetail);
