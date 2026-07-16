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
}
function isPreviewableImage(source) {
  if (/^data:image\/(?:avif|gif|jpeg|png|webp);base64,[a-z0-9+/=\s]+$/i.test(source)) return true;
  try {
    const url = new URL(source);
    return ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password;
  } catch (_) { return false; }
}
function localPathLabel(source) { return source.replace(/\\/g, '/').split('/').filter(Boolean).pop() || source; }
function isAbsoluteLocalPath(path) { return path.startsWith('/') || /^[a-z]:[\\/]/i.test(path) || path.startsWith('\\\\'); }
function renderImagePreview() {
  const container = document.querySelector('#image-preview');
  container.replaceChildren();
  mediaLines(document.querySelector('[name="images"]').value).forEach((source, index) => {
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
      item.textContent = `本地素材：${localPathLabel(source)}（仅服务端读取）`;
    }
    container.append(item);
  });
}
function renderVideoPath() {
  const placeholder = document.querySelector('#video-preview');
  const path = document.querySelector('[name="video"]').value.trim();
  placeholder.hidden = !path;
  placeholder.textContent = path ? `本地视频：${localPathLabel(path)}（仅服务端读取）` : '';
}
function updateCount(input, output, maximum) {
  const count = characterCount(input.value);
  output.textContent = `${count}/${maximum}`;
  output.classList.toggle('over-limit', count > maximum);
}
function confirmationMessage(body, mode) {
  const account = XHS.currentAccount();
  const accountLabel = account ? `${account.display_name} (${account.id})` : XHS.state.selectedAccountId;
  const material = mode === 'image' ? `${body.images.length} 张图片` : '1 个本地视频';
  const timing = body.schedule_at ? `定时发布：${body.schedule_at}` : '立即发布';
  return `发布操作将提交到小红书，请再次确认：\n\n当前账号：${accountLabel}\n发布模式：${mode === 'image' ? '图文笔记' : '视频笔记'}\n标题：${body.title}\n素材数量：${material}\n发布时间：${timing}`;
}
async function publish(event) {
  event.preventDefault(); if (!XHS.requireAccount()) return;
  const form = new FormData(event.currentTarget), mode = document.body.dataset.publishMode || 'image';
  const title = String(form.get('title') || '').trim(), content = String(form.get('content') || '').trim();
  try {
    if (!title || characterCount(title) > 20) throw new Error('标题必填且不能超过 20 个字符');
    if (characterCount(content) > 1000) throw new Error('正文不能超过 1000 个字符');
    validateSchedule(form.get('schedule_at'));
    const body = {title, content, tags:lines(form.get('tags')), products:lines(form.get('products')), visibility:form.get('visibility'), schedule_at:scheduleISO(form.get('schedule_at'))};
    if (mode === 'image') { body.images = mediaLines(form.get('images')); body.is_original = form.get('is_original') === 'on'; if (!body.images.length) throw new Error('图文发布至少需要一张图片'); }
    else {
      body.video = String(form.get('video') || '').trim();
      if (!body.video || !isAbsoluteLocalPath(body.video)) throw new Error('请输入服务端本地视频绝对路径');
    }
    if (!window.confirm(confirmationMessage(body, mode))) return;
    XHS.loading(true, mode === 'image' ? '正在发布图文…' : '正在发布视频…');
    const data = await XHS.callTool(mode === 'image' ? 'publish_content' : 'publish_with_video', body);
    XHS.toast(data.status || '发布完成');
  } catch (error) { XHS.toast(error.message, 'error'); } finally { XHS.loading(false); }
}
document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('[data-mode]').forEach(button => button.addEventListener('click', () => setMode(button.dataset.mode)));
  document.querySelector('#publish-form').addEventListener('submit', publish);
  const title = document.querySelector('[name="title"]'), content = document.querySelector('[name="content"]');
  title.addEventListener('input', () => updateCount(title, document.querySelector('#title-count'), 20));
  content.addEventListener('input', () => updateCount(content, document.querySelector('#content-count'), 1000));
  document.querySelector('[name="images"]').addEventListener('input', renderImagePreview);
  document.querySelector('[name="video"]').addEventListener('input', renderVideoPath);
  updateCount(title, document.querySelector('#title-count'), 20);
  updateCount(content, document.querySelector('#content-count'), 1000);
  setMode('image');
});
