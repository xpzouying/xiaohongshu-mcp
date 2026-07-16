const params = new URLSearchParams(location.search);
const detailState = {feedId: params.get('feed_id') || '', token: params.get('xsec_token') || '', note: null, controller: null};
function imageURL(image) { return image.urlDefault || image.urlPre || image.url || ''; }
function safeImageURL(value) {
  try {
    const url = new URL(value, location.origin);
    return ['http:', 'https:'].includes(url.protocol) ? XHS.escapeHTML(url.href) : '';
  } catch (_) { return ''; }
}
function profileURL(user) {
  const userId = user.userId || user.user_id || user.id || '';
  return userId ? `/profile.html?user_id=${encodeURIComponent(userId)}&xsec_token=${encodeURIComponent(detailState.token)}` : '';
}
function renderUser(user, fallback = '用户') {
  const name = XHS.escapeHTML(user.nickname || user.nickName || fallback);
  const url = profileURL(user);
  return url ? `<a href="${url}"><strong>${name}</strong></a>` : `<strong>${name}</strong>`;
}
function renderComment(comment, child = false) {
  const user = comment.userInfo || comment.user || {};
  const userId = user.userId || user.user_id || user.id || '';
  return `<article class="comment${child ? ' sub-comment' : ''}"><div>${renderUser(user)}<small>${XHS.escapeHTML(comment.ipLocation || '')}</small></div><p>${XHS.escapeHTML(comment.content || '')}</p><button class="link-button reply-button" data-comment-id="${XHS.escapeHTML(comment.id || '')}" data-user-id="${XHS.escapeHTML(userId)}">回复</button>${(comment.subComments || []).map(item => renderComment(item, true)).join('')}</article>`;
}
function renderDetail(payload) {
  const data = payload.data || payload; const note = data.note || {}; detailState.note = note;
  const user = note.user || {}, info = note.interactInfo || {};
  const authorURL = profileURL(user), avatar = safeImageURL(user.avatar || '');
  document.querySelector('#detail-content').innerHTML = `<div class="author">${authorURL ? `<a href="${authorURL}">` : ''}${avatar ? `<img src="${avatar}" alt="">` : ''}${authorURL ? '</a>' : ''}<div>${renderUser(user, '未知作者')}<small>${XHS.escapeHTML(note.ipLocation || '')}</small></div></div><h1>${XHS.escapeHTML(note.title || '无标题')}</h1><p class="note-desc">${XHS.escapeHTML(note.desc || '')}</p><div class="image-gallery">${(note.imageList || []).map(image => safeImageURL(imageURL(image))).filter(Boolean).map(url => `<img src="${url}" alt="笔记图片" loading="lazy">`).join('')}</div><div class="interaction-bar"><button id="like-button" class="${info.liked ? 'active' : ''}">♥ ${XHS.escapeHTML(info.likedCount || '0')}</button><button id="favorite-button" class="${info.collected ? 'active' : ''}">☆ ${XHS.escapeHTML(info.collectedCount || '0')}</button><span>评论 ${XHS.escapeHTML(info.commentCount || '0')}</span><span>分享 ${XHS.escapeHTML(info.sharedCount || '0')}</span></div>`;
  document.querySelector('#comments').innerHTML = (data.comments?.list || []).map(item => renderComment(item)).join('') || '<p class="empty">暂无评论</p>';
  document.querySelector('#like-button').addEventListener('click', event => toggleAction('like', info.liked, event.currentTarget));
  document.querySelector('#favorite-button').addEventListener('click', event => toggleAction('favorite', info.collected, event.currentTarget));
}
function detailOptions() {
  const controls = document.querySelector('#comment-options-form').elements;
  return {
    load_all_comments: controls.load_all_comments.checked,
    click_more_replies: controls.click_more_replies.checked,
    limit: Number(controls.limit.value) || 20,
    reply_limit: Number(controls.reply_limit.value) || 10,
    scroll_speed: controls.scroll_speed.value || 'normal'
  };
}
function syncDetailOptions(form = document.querySelector('#comment-options-form')) {
  const loadAll = form.elements.load_all_comments;
  const clickReplies = form.elements.click_more_replies;
  form.elements.limit.disabled = !loadAll.checked;
  clickReplies.disabled = !loadAll.checked;
  form.elements.scroll_speed.disabled = !loadAll.checked;
  form.elements.reply_limit.disabled = !loadAll.checked || !clickReplies.checked;
  if (!loadAll.checked) clickReplies.checked = false;
}
function setPending(pending, form = document.querySelector('#comment-options-form')) {
  form.querySelectorAll('button, input, select, textarea').forEach(control => { control.disabled = pending; });
  if (form.id === 'comment-options-form') {
    document.querySelector('#cancel-detail').hidden = !pending;
    document.querySelector('#cancel-detail').disabled = false;
    if (!pending) syncDetailOptions(form);
  }
}
function showDetailError(message = '') {
  const error = document.querySelector('#detail-error');
  error.textContent = message;
  error.hidden = !message;
}
async function loadDetail() {
  if (!detailState.feedId || !detailState.token) { document.querySelector('#detail-content').innerHTML = '<div class="empty">请从搜索结果进入，或在地址中提供 feed_id 与 xsec_token。</div>'; return; }
  if (!XHS.requireAccount()) return;
  detailState.controller?.abort();
  detailState.controller = new AbortController();
  const controller = detailState.controller;
  const options = detailOptions();
  setPending(true); showDetailError(''); XHS.loading(true, '正在加载笔记…');
  try { const data = await XHS.callTool('get_feed_detail', {feed_id:detailState.feedId, xsec_token:detailState.token, ...options}, {signal:controller.signal}); renderDetail(data); }
  catch (error) {
    if (detailState.controller === controller) {
      const message = error.name === 'AbortError' ? '已取消加载笔记' : error.message;
      showDetailError(message); XHS.toast(message, error.name === 'AbortError' ? 'warning' : 'error');
    }
  }
  finally { if (detailState.controller === controller) { detailState.controller = null; setPending(false); XHS.loading(false); } }
}
function validateReply(form) {
  const commentId = String(form.get('comment_id') || '').trim();
  const userId = String(form.get('user_id') || '').trim();
  const content = String(form.get('content') || '').trim();
  if ((!commentId && !userId) || !content) return null;
  return {comment_id:commentId, user_id:userId, content};
}
async function toggleAction(action, active, button) {
  button.disabled = true;
  const tool = action === 'like' ? 'like_feed' : 'favorite_feed';
  try { await XHS.callTool(tool, {feed_id:detailState.feedId, xsec_token:detailState.token, [action === 'like' ? 'unlike' : 'unfavorite']:active}); XHS.toast(active ? '已取消' : '操作成功'); await loadDetail(); }
  catch (error) { XHS.toast(error.message, 'error'); }
  finally { button.disabled = false; }
}
document.addEventListener('DOMContentLoaded', () => {
  const optionsForm = document.querySelector('#comment-options-form');
  const loadAll = optionsForm.elements.load_all_comments;
  const clickReplies = optionsForm.elements.click_more_replies;
  loadAll.addEventListener('change', () => syncDetailOptions(optionsForm));
  clickReplies.addEventListener('change', () => syncDetailOptions(optionsForm));
  optionsForm.addEventListener('submit', event => { event.preventDefault(); loadDetail(); });
  document.querySelector('#cancel-detail').addEventListener('click', () => detailState.controller?.abort());
  syncDetailOptions(optionsForm);
  document.querySelector('#comment-form').addEventListener('submit', async event => {
    event.preventDefault(); const content = new FormData(event.currentTarget).get('content').trim(); if (!content) return;
    setPending(true, event.currentTarget);
    try { await XHS.callTool('post_comment_to_feed', {feed_id:detailState.feedId, xsec_token:detailState.token, content}); event.currentTarget.reset(); XHS.toast('评论发表成功'); await loadDetail(); } catch (error) { XHS.toast(error.message, 'error'); } finally { setPending(false, event.currentTarget); }
  });
  document.querySelector('#comments').addEventListener('click', event => {
    const button = event.target.closest('.reply-button'); if (!button) return;
    document.querySelector('#reply-comment-id').value = button.dataset.commentId;
    document.querySelector('#reply-user-id').value = button.dataset.userId;
    document.querySelector('#reply-dialog').showModal();
  });
  document.querySelector('#reply-form').addEventListener('submit', async event => {
    event.preventDefault(); const formElement = event.currentTarget, form = new FormData(formElement);
    const reply = validateReply(form); if (!reply) { XHS.toast('请输入回复内容并选择目标评论', 'warning'); return; }
    setPending(true, formElement);
    try { await XHS.callTool('reply_comment_in_feed', {feed_id:detailState.feedId,xsec_token:detailState.token,...reply}); document.querySelector('#reply-dialog').close(); formElement.reset(); XHS.toast('回复成功'); await loadDetail(); } catch (error) { XHS.toast(error.message, 'error'); } finally { setPending(false, formElement); }
  });
  document.querySelector('#cancel-reply').addEventListener('click', () => document.querySelector('#reply-dialog').close());
});
window.addEventListener('accountsready', loadDetail);
window.addEventListener('accountchange', loadDetail);
