function lines(value) { return String(value || '').split(/\n|,/).map(item => item.trim()).filter(Boolean); }
function mediaLines(value) { return String(value || '').split(/\r?\n/).map(item => item.trim()).filter(Boolean); }
function characterCount(value) { return [...String(value || '')].length; }
function scheduleISO(value) { return value ? new Date(value).toISOString() : ''; }
function validateSchedule(value) {
  if (!value) return;
  const time = new Date(value).getTime(), now = Date.now();
  if (time < now + 60 * 60 * 1000 || time > now + 14 * 24 * 60 * 60 * 1000) throw new Error('定时发布时间须在 1 小时至 14 天内');
}
function setMode(mode) {
  document.body.dataset.publishMode = mode;
  document.querySelectorAll('[data-mode]').forEach(button => button.classList.toggle('active', button.dataset.mode === mode));
  document.querySelector('#images-field').hidden = mode !== 'image';
  document.querySelector('#video-field').hidden = mode !== 'video';
  document.querySelector('[name="is_original"]').closest('label').hidden = mode !== 'image';
}
function isPreviewableImage(source) {
  if (/^data:image\/(?:avif|gif|jpeg|png|webp);base64,[a-z0-9+/=\s]+$/i.test(source)) return true;
  try {
    const url = new URL(source);
    return ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password;
  } catch (_) { return false; }
}
function localPathLabel(source) { return source.replace(/\\/g, '/').split('/').filter(Boolean).pop() || source; }
function fileExtension(source) {
  const name = localPathLabel(source), position = name.lastIndexOf('.');
  return position > 0 ? name.slice(position).toLowerCase() : '无扩展名';
}
function isAbsoluteLocalPath(path) { return path.startsWith('/') || /^[a-z]:[\\/]/i.test(path) || path.startsWith('\\\\'); }
function appendImagePreview(container, sources, confirmation = false) {
  container.replaceChildren();
  sources.forEach((source, index) => {
    const item = document.createElement('div');
    item.className = 'media-preview-item';
    if (isPreviewableImage(source)) {
      const image = document.createElement('img');
      image.alt = `图片 ${index + 1} 预览`;
      image.loading = 'lazy';
      image.referrerPolicy = 'no-referrer';
      image.src = source;
      item.append(image);
    } else {
      item.classList.add('local-media-placeholder');
      item.textContent = `服务端本地路径：${source}（浏览器无法预览）`;
    }
    if (confirmation) item.setAttribute('aria-label', `素材 ${index + 1}`);
    container.append(item);
  });
}
function renderImagePreview() {
  appendImagePreview(document.querySelector('#image-preview'), mediaLines(document.querySelector('[name="images"]').value));
}
function renderVideoPath() {
  const placeholder = document.querySelector('#video-preview');
  const path = document.querySelector('[name="video"]').value.trim();
  placeholder.hidden = !path;
  placeholder.textContent = path ? `服务端绝对路径：${path}；文件名：${localPathLabel(path)}；扩展名：${fileExtension(path)}。浏览器不提供本地视频播放预览。` : '';
}
function updateCount(input, output, maximum) {
  const count = characterCount(input.value);
  output.textContent = `${count}/${maximum}`;
  output.classList.toggle('over-limit', count > maximum);
}
function contentSummary(content, maximum = 160) {
  const compact = String(content || '').replace(/\s+/g, ' ').trim();
  return characterCount(compact) > maximum ? `${[...compact].slice(0, maximum).join('')}…` : compact || '（空）';
}
function confirmationDetails(body, mode, accountLabel) {
  return [
    ['当前账号', accountLabel || '（未选择）'],
    ['发布模式', mode === 'image' ? '图文笔记' : '视频笔记'],
    ['标题', body.title],
    ['正文摘要', contentSummary(body.content)],
    ['标签', body.tags.length ? body.tags.join('、') : '（无）'],
    ['商品', body.products.length ? body.products.join('、') : '（无）'],
    ['可见范围', body.visibility || '公开可见'],
    ['发布时间', body.schedule_at ? `定时：${body.schedule_at}` : '立即发布'],
    ['素材数量', mode === 'image' ? `${body.images.length} 张图片` : '1 个视频']
  ];
}
function confirmationHTML(body, mode, accountLabel) {
  return confirmationDetails(body, mode, accountLabel).map(([label, value]) =>
    `<div class="confirmation-row"><dt>${XHS.escapeHTML(label)}</dt><dd>${XHS.escapeHTML(value)}</dd></div>`
  ).join('');
}
function buildPayload(form, mode) {
  const title = String(form.get('title') || '').trim(), content = String(form.get('content') || '').trim();
  if (!title || characterCount(title) > 20) throw new Error('标题必填且不能超过 20 个字符');
  if (characterCount(content) > 1000) throw new Error('正文不能超过 1000 个字符');
  validateSchedule(form.get('schedule_at'));
  const body = {title, content, tags:lines(form.get('tags')), products:lines(form.get('products')), visibility:form.get('visibility'), schedule_at:scheduleISO(form.get('schedule_at'))};
  if (mode === 'image') {
    body.images = mediaLines(form.get('images'));
    body.is_original = form.get('is_original') === 'on';
    if (!body.images.length) throw new Error('图文发布至少需要一张图片');
  } else {
    body.video = String(form.get('video') || '').trim();
    if (!body.video || !isAbsoluteLocalPath(body.video)) throw new Error('请输入服务端本地视频绝对路径');
  }
  return body;
}

let pendingConfirmation = null;
let publishing = false;

function openConfirmation(body, mode) {
  const account = XHS.currentAccount();
  const accountId = XHS.state.selectedAccountId;
  const accountLabel = account ? `${account.display_name} (${account.id})` : accountId;
  pendingConfirmation = {body, mode, accountId, accountLabel};
  document.querySelector('#confirmation-summary').innerHTML = confirmationHTML(body, mode, accountLabel);
  const media = document.querySelector('#confirmation-media');
  if (mode === 'image') {
    appendImagePreview(media, body.images, true);
  } else {
    media.replaceChildren();
    const item = document.createElement('div');
    item.className = 'local-media-placeholder video-confirmation';
    item.textContent = `服务端绝对路径：${body.video}；文件名：${localPathLabel(body.video)}；扩展名：${fileExtension(body.video)}。浏览器不提供本地视频播放预览。`;
    media.append(item);
  }
  document.querySelector('#publish-confirmation').showModal();
}
function closeConfirmation() {
  if (publishing) return;
  pendingConfirmation = null;
  document.querySelector('#publish-confirmation').close();
}
function invalidateConfirmationOnAccountChange() {
  if (publishing || !pendingConfirmation) return;
  pendingConfirmation = null;
  document.querySelector('#publish-confirmation').close();
  XHS.toast('当前账号已变化，请返回表单重新确认', 'warning');
}
async function confirmPublish() {
  if (publishing || !pendingConfirmation) return;
  const confirmation = pendingConfirmation;
  pendingConfirmation = null;
  document.querySelector('#publish-confirmation').close();
  if (!confirmation.accountId || confirmation.accountId !== XHS.state.selectedAccountId) {
    XHS.toast('当前账号已变化，请返回表单重新确认', 'warning');
    return;
  }
  publishing = true;
  const {body, mode, accountId} = confirmation;
  const confirmButton = document.querySelector('#confirm-publish');
  confirmButton.disabled = true;
  try {
    XHS.loading(true, mode === 'image' ? '正在发布图文…' : '正在发布视频…');
    const data = await XHS.callTool(mode === 'image' ? 'publish_content' : 'publish_with_video', body, {accountId});
    XHS.toast(data.status || '发布完成');
  } catch (error) {
    const timeout = error?.status === 504 || /TIMEOUT/i.test(error?.code || '') || /超时|timed?\s*out/i.test(error?.message || '');
    XHS.toast(timeout
      ? '发布结果 UNKNOWN：请求已超时，不会自动重试。请核实发布结果后返回表单重新确认。'
      : `${error.message}。请返回表单重新确认后再试。`, 'error');
  } finally {
    publishing = false;
    confirmButton.disabled = false;
    XHS.loading(false);
  }
}
function publish(event) {
  event.preventDefault();
  if (!XHS.requireAccount() || publishing) return;
  try {
    const mode = document.body.dataset.publishMode || 'image';
    openConfirmation(buildPayload(new FormData(event.currentTarget), mode), mode);
  } catch (error) { XHS.toast(error.message, 'error'); }
}
document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('[data-mode]').forEach(button => button.addEventListener('click', () => setMode(button.dataset.mode)));
  document.querySelector('#publish-form').addEventListener('submit', publish);
  document.querySelector('#cancel-publish').addEventListener('click', closeConfirmation);
  document.querySelector('#confirm-publish').addEventListener('click', confirmPublish);
  document.querySelector('#publish-confirmation').addEventListener('cancel', event => { event.preventDefault(); closeConfirmation(); });
  const title = document.querySelector('[name="title"]'), content = document.querySelector('[name="content"]');
  title.addEventListener('input', () => updateCount(title, document.querySelector('#title-count'), 20));
  content.addEventListener('input', () => updateCount(content, document.querySelector('#content-count'), 1000));
  document.querySelector('[name="images"]').addEventListener('input', renderImagePreview);
  document.querySelector('[name="video"]').addEventListener('input', renderVideoPath);
  updateCount(title, document.querySelector('#title-count'), 20);
  updateCount(content, document.querySelector('#content-count'), 1000);
  setMode('image');
});
window.addEventListener('accountchange', invalidateConfirmationOnAccountChange);
