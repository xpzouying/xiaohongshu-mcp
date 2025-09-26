package xiaohongshu

import (
    "context"
    "log/slog"
    "os"
    "time"

    "github.com/go-rod/rod"
    "github.com/go-rod/rod/lib/proto"
    "github.com/pkg/errors"
)

// PublishVideoContent 发布视频内容
type PublishVideoContent struct {
    Title     string
    Content   string
    Tags      []string
    VideoPath string
}

// NewPublishVideoAction 进入发布页并选择“上传视频”
func NewPublishVideoAction(page *rod.Page) (*PublishAction, error) {
    pp := page.Timeout(60 * time.Second)

    pp.MustNavigate(urlOfPublic)
    pp.MustElement(`div.upload-content`).MustWaitVisible()
    slog.Info("wait for upload-content visible success")

    time.Sleep(1 * time.Second)

    createElems := pp.MustElements("div.creator-tab")
    var visibleElems []*rod.Element
    for _, elem := range createElems {
        if isElementVisible(elem) {
            visibleElems = append(visibleElems, elem)
        }
    }
    if len(visibleElems) == 0 {
        return nil, errors.New("没有找到上传视频元素")
    }
    for _, elem := range visibleElems {
        text, err := elem.Text()
        if err != nil {
            slog.Error("获取元素文本失败", "error", err)
            continue
        }
        if text == "上传视频" {
            if err := elem.Click(proto.InputMouseButtonLeft, 1); err != nil {
                slog.Error("点击元素失败", "error", err)
                continue
            }
            break
        }
    }

    time.Sleep(1 * time.Second)

    return &PublishAction{page: pp}, nil
}

// PublishVideo 执行上传和提交
func (p *PublishAction) PublishVideo(ctx context.Context, content PublishVideoContent) error {
    if content.VideoPath == "" {
        return errors.New("视频不能为空")
    }
    if _, err := os.Stat(content.VideoPath); os.IsNotExist(err) {
        return errors.Wrapf(err, "视频文件不存在: %s", content.VideoPath)
    }

    page := p.page.Context(ctx)
    if err := uploadVideo(page, content.VideoPath); err != nil {
        return errors.Wrap(err, "小红书上传视频失败")
    }

    // 等待一段时间确保视频处理完成，按钮可点击
    time.Sleep(20 * time.Second)

    if err := submitPublish(page, content.Title, content.Content, content.Tags); err != nil {
        return errors.Wrap(err, "小红书发布失败")
    }
    return nil
}

func uploadVideo(page *rod.Page, videoPath string) error {
    pp := page.Timeout(5 * time.Minute)

    // 等待上传输入框出现
    // 与图文一致的上传控件选择器，通常为 .upload-input
    uploadInput, err := pp.Element(".upload-input")
    if err != nil || uploadInput == nil {
        // fallback：查找 file input
        uploadInput, err = pp.Element(`input[type="file"]`)
        if err != nil || uploadInput == nil {
            return errors.New("未找到视频上传输入框")
        }
    }

    uploadInput.MustSetFiles(videoPath)

    return waitForVideoUploadComplete(pp)
}

// waitForVideoUploadComplete 等待视频上传完成
func waitForVideoUploadComplete(page *rod.Page) error {
    maxWait := 5 * time.Minute
    interval := 1 * time.Second
    start := time.Now()

    slog.Info("开始等待视频上传完成（cover-container: analyze -> uploading -> preview-new(reupload)）")

    for time.Since(start) < maxWait {
        // 定位封面容器
        container, _ := page.Element(".cover-container")
        if container == nil {
            // 容器还没出现，继续等待
            time.Sleep(interval)
            continue
        }

        // 成功条件：出现 preview-new，且其内部包含 .reupload
        previews, _ := container.Elements(".preview-new")
        if len(previews) > 0 {
            for _, pv := range previews {
                if pv == nil {
                    continue
                }
                if child, _ := pv.Element(".reupload"); child != nil {
                    slog.Info("检测到 preview-new 内的 reupload，视频上传已完成")
                    return nil
                }
            }
            // 预览已出现但未见 reupload，继续等待
            slog.Debug("preview-new 存在但未检测到 reupload，继续等待")
        }

        // 进行中条件：存在 uploading
        uploading, _ := container.Elements(".uploading")
        if len(uploading) > 0 {
            slog.Debug("视频正在上传中 (uploading 存在)")
            time.Sleep(interval)
            continue
        }

        // 进行中条件：存在 analyze
        analyzing, _ := container.Elements(".analyze")
        if len(analyzing) > 0 {
            // 仍在分析/上传中
            slog.Debug("视频仍在上传/分析中 (analyze 存在)")
            time.Sleep(interval)
            continue
        }

        // 若既无 preview-new 也无 analyze，则可能DOM尚未更新或选择器变化，继续轮询
        slog.Debug("未检测到 analyze 或 preview-new，继续等待")
        time.Sleep(interval)
    }

    return errors.New("上传视频超时（未出现 preview-new），请检查网络或视频大小")
}
