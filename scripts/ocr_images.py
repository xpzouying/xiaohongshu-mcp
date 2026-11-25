"""从标准输入接收笔记图片 URL，按原图顺序返回本地识别的文字。"""

import contextlib
import io
import json
import sys
from urllib.parse import urlsplit, urlunsplit
from urllib.request import HTTPRedirectHandler, Request, build_opener


def secure_image_url(url):
    parsed = urlsplit(url)
    host = (parsed.hostname or "").lower()
    if parsed.scheme not in ("http", "https") or not host.endswith(".xhscdn.com"):
        raise ValueError("仅支持小红书 CDN 图片地址")
    if parsed.username or parsed.password or parsed.port not in (None, 443):
        raise ValueError("图片地址格式不正确")
    return urlunsplit(("https", parsed.netloc, parsed.path, parsed.query, ""))


class CDNRedirectHandler(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        # 在发出下一跳请求之前校验，避免先访问非 CDN 地址再检查。
        return super().redirect_request(
            req, fp, code, msg, headers, secure_image_url(newurl)
        )


def read_image(url):
    request = Request(secure_image_url(url), headers={
        "User-Agent": "Mozilla/5.0",
        "Referer": "https://www.xiaohongshu.com/",
    })
    with build_opener(CDNRedirectHandler()).open(request, timeout=20) as response:
        data = response.read(20 * 1024 * 1024 + 1)
    if len(data) > 20 * 1024 * 1024:
        raise ValueError("图片超过20MB，未识别")
    return data


def recognize_images(urls):
    from PIL import Image, ImageOps
    from rapidocr import RapidOCR

    if not urls:
        return {"images": []}
    engine = RapidOCR(params={
        "Global.log_level": "error",
        "Global.max_side_len": 4096,
        "EngineConfig.onnxruntime.intra_op_num_threads": 2,
        "EngineConfig.onnxruntime.inter_op_num_threads": 2,
    })
    images = []
    for index, url in enumerate(urls, 1):
        item = {"index": index, "url": url, "text": "", "confidence": 0.0}
        try:
            with Image.open(io.BytesIO(read_image(url))) as source:
                image = ImageOps.exif_transpose(source).convert("RGB")
            result = engine(image)
            texts = result.txts if result.txts is not None else ()
            scores = result.scores if result.scores is not None else ()
            item["lines"] = [
                {"text": text, "confidence": round(float(score), 4)}
                for text, score in zip(texts, scores)
            ]
            item["text"] = "\n".join(line["text"] for line in item["lines"])
            if item["lines"]:
                item["confidence"] = round(sum(line["confidence"] for line in item["lines"]) / len(item["lines"]), 4)
        except Exception as exc:
            item["error"] = f"{type(exc).__name__}: {exc}"
        images.append(item)
    return {"images": images}


def main():
    sys.stdin.reconfigure(encoding="utf-8")
    sys.stdout.reconfigure(encoding="utf-8")
    try:
        request = json.load(sys.stdin)
        urls = request["urls"]
        if not isinstance(urls, list) or not all(isinstance(url, str) for url in urls):
            raise ValueError("urls必须是图片地址数组")
        # 依赖库的日志不能混进标准输出的 JSON 协议。
        with contextlib.redirect_stdout(sys.stderr):
            result = recognize_images(urls)
    except Exception as exc:
        result = {"error": f"{type(exc).__name__}: {exc}"}
    json.dump(result, sys.stdout, ensure_ascii=False)


if __name__ == "__main__":
    main()
