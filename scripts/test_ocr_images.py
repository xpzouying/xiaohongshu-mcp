import io
import unittest
from types import SimpleNamespace
from unittest.mock import patch
from urllib.request import Request

from PIL import Image

from ocr_images import CDNRedirectHandler, read_image, recognize_images, secure_image_url


class OCRImagesTests(unittest.TestCase):
    def test_rejects_non_cdn_and_local_addresses(self):
        for url in ("file:///tmp/image.png", "http://127.0.0.1/x", "https://xhscdn.com.evil.test/x",
                    "https://user:password@a.xhscdn.com/x", "https://a.xhscdn.com:8080/x"):
            with self.subTest(url=url), self.assertRaises(ValueError):
                read_image(url)

    def test_http_cdn_address_is_upgraded_to_https(self):
        self.assertEqual(secure_image_url("http://a.xhscdn.com/image?token=example#fragment"),
                         "https://a.xhscdn.com/image?token=example")

    def test_redirect_is_validated_before_following_it(self):
        handler = CDNRedirectHandler()
        req = Request("https://a.xhscdn.com/image")
        with self.assertRaises(ValueError):
            handler.redirect_request(req, None, 302, "Found", {}, "http://127.0.0.1/private")
        redirected = handler.redirect_request(req, None, 302, "Found", {}, "http://b.xhscdn.com/image")
        self.assertEqual(redirected.full_url, "https://b.xhscdn.com/image")

    def test_oversized_image_is_rejected(self):
        with patch("ocr_images.build_opener") as opener:
            opener.return_value.open.return_value.__enter__.return_value.read.return_value = b"x" * (20 * 1024 * 1024 + 1)
            with self.assertRaisesRegex(ValueError, "20MB"):
                read_image("https://a.xhscdn.com/image")

    def test_partial_failure_and_empty_image_keep_original_positions(self):
        data = io.BytesIO()
        Image.new("RGB", (40, 40), "white").save(data, format="PNG")
        outputs = [
            SimpleNamespace(txts=("HashMap", "HashMap"), scores=(0.98, 0.81)),
            SimpleNamespace(txts=None, scores=None),
        ]
        with patch("rapidocr.RapidOCR") as factory, patch("ocr_images.read_image") as download:
            factory.return_value.side_effect = outputs
            download.side_effect = [OSError("图片过期"), data.getvalue(), data.getvalue()]
            result = recognize_images(["first", "second", "third"])["images"]
        self.assertEqual([item["index"] for item in result], [1, 2, 3])
        self.assertIn("图片过期", result[0]["error"])
        self.assertEqual(result[1]["text"], "HashMap\nHashMap")
        self.assertEqual(result[1]["lines"][1]["confidence"], 0.81)
        self.assertEqual(result[2]["text"], "")
        self.assertNotIn("error", result[2])


if __name__ == "__main__":
    unittest.main()
