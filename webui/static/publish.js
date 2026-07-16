function lines(value) { return String(value || '').split(/\n|,/).map(item => item.trim()).filter(Boolean); }
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
async function publish(event) {
  event.preventDefault(); if (!XHS.requireAccount()) return;
  const form = new FormData(event.currentTarget), mode = document.body.dataset.publishMode || 'image';
  const title = String(form.get('title') || '').trim(), content = String(form.get('content') || '').trim();
  try {
    if (!title || [...title].length > 20) throw new Error('标题必填且不能超过 20 个字符');
    if ([...content].length > 1000) throw new Error('正文不能超过 1000 个字符');
    validateSchedule(form.get('schedule_at'));
    const body = {title, content, tags:lines(form.get('tags')), products:lines(form.get('products')), visibility:form.get('visibility'), schedule_at:scheduleISO(form.get('schedule_at'))};
    if (mode === 'image') { body.images = lines(form.get('images')); body.is_original = form.get('is_original') === 'on'; if (!body.images.length) throw new Error('图文发布至少需要一张图片'); }
    else { body.video = String(form.get('video') || '').trim(); if (!body.video) throw new Error('请输入本地视频绝对路径'); }
    XHS.loading(true, mode === 'image' ? '正在发布图文…' : '正在发布视频…');
    const data = await XHS.api(mode === 'image' ? '/api/web/publish' : '/api/web/publish_video', {method:'POST', body});
    XHS.toast(data.status || '发布完成');
  } catch (error) { XHS.toast(error.message, 'error'); } finally { XHS.loading(false); }
}
document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('[data-mode]').forEach(button => button.addEventListener('click', () => setMode(button.dataset.mode)));
  document.querySelector('#publish-form').addEventListener('submit', publish);
  setMode('image');
});
