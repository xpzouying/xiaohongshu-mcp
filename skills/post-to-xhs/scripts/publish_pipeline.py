"""
Unified publish pipeline for Xiaohongshu.

Single CLI entry point that orchestrates:
  chrome_launcher → login check → image download → form fill → (optional) publish

Usage:
    # Fill form only (default) - review in browser before publishing
    python publish_pipeline.py --title "标题" --content "正文" --image-urls URL1 URL2
    python publish_pipeline.py --title-file t.txt --content-file body.txt --image-urls URL1

    # Headless mode (no GUI window) - faster for automated publishing
    python publish_pipeline.py --headless --title-file t.txt --content-file body.txt --image-urls URL1

    # Publish to a specific account
    python publish_pipeline.py --account myaccount --title "标题" --content "正文" --image-urls URL1

    # Fill and auto-publish in one step
    python publish_pipeline.py --title "标题" --content "正文" --image-urls URL1 --auto-publish

    # Use local image files instead of URLs
    python publish_pipeline.py --title "标题" --content "正文" --images img1.jpg img2.jpg

Exit codes:
    0 = success (READY_TO_PUBLISH or PUBLISHED)
    1 = not logged in (NOT_LOGGED_IN) - headless auto-fallback will restart headed
    2 = error (see stderr)
"""

import argparse
import os
import sys

# Ensure UTF-8 output on Windows consoles
if sys.platform == "win32":
    os.environ.setdefault("PYTHONIOENCODING", "utf-8")
    try:
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
        sys.stderr.reconfigure(encoding="utf-8", errors="replace")
    except Exception:
        pass

# Add scripts dir to path so sibling modules can be imported
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
if SCRIPT_DIR not in sys.path:
    sys.path.insert(0, SCRIPT_DIR)

from chrome_launcher import ensure_chrome, restart_chrome
from cdp_publish import XiaohongshuPublisher, CDPError
from image_downloader import ImageDownloader


def main():
    parser = argparse.ArgumentParser(
        description="Xiaohongshu publish pipeline - unified entry point"
    )

    # Title
    title_group = parser.add_mutually_exclusive_group(required=True)
    title_group.add_argument("--title", help="Article title text")
    title_group.add_argument("--title-file", help="Read title from UTF-8 file")

    # Content
    content_group = parser.add_mutually_exclusive_group(required=True)
    content_group.add_argument("--content", help="Article body text")
    content_group.add_argument("--content-file", help="Read content from UTF-8 file")

    # Images
    img_group = parser.add_mutually_exclusive_group(required=True)
    img_group.add_argument(
        "--image-urls", nargs="+", help="Image URLs to download"
    )
    img_group.add_argument(
        "--images", nargs="+", help="Local image file paths"
    )

    # Publish mode
    parser.add_argument(
        "--auto-publish",
        action="store_true",
        default=False,
        help="Click publish button after filling (default: fill only)",
    )

    # Headless mode
    parser.add_argument(
        "--headless",
        action="store_true",
        default=False,
        help="Run Chrome in headless mode (no GUI). Auto-falls back to headed if login is needed.",
    )

    # Optional temp dir for downloaded images
    parser.add_argument(
        "--temp-dir",
        default=None,
        help="Directory for downloaded images (default: auto-created temp dir)",
    )

    # Account selection
    parser.add_argument(
        "--account",
        default=None,
        help="Account name to publish to (default: default account)",
    )

    args = parser.parse_args()
    headless = args.headless
    account = args.account

    # --- Resolve title ---
    if args.title_file:
        with open(args.title_file, encoding="utf-8") as f:
            title = f.read().strip()
    else:
        title = args.title

    if not title:
        print("Error: title is empty.", file=sys.stderr)
        sys.exit(2)

    # --- Resolve content ---
    if args.content_file:
        with open(args.content_file, encoding="utf-8") as f:
            content = f.read().strip()
    else:
        content = args.content

    if not content:
        print("Error: content is empty.", file=sys.stderr)
        sys.exit(2)

    # --- Step 1: Ensure Chrome is running ---
    mode_label = "headless" if headless else "headed"
    account_label = account or "default"
    print(f"[pipeline] Step 1: Ensuring Chrome is running ({mode_label}, account: {account_label})...")
    if not ensure_chrome(headless=headless, account=account):
        print("Error: Failed to start Chrome.", file=sys.stderr)
        sys.exit(2)

    # --- Step 2: Connect and check login ---
    print("[pipeline] Step 2: Checking login status...")
    publisher = XiaohongshuPublisher()
    try:
        publisher.connect()
        logged_in = publisher.check_login()
        if not logged_in:
            publisher.disconnect()
            if headless:
                # Auto-fallback: restart Chrome in headed mode for QR login
                print("[pipeline] Headless mode: not logged in. Switching to headed mode for login...")
                restart_chrome(headless=False, account=account)
                publisher.connect()
                publisher.open_login_page()
            print("NOT_LOGGED_IN")
            sys.exit(1)
    except CDPError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(2)

    # --- Step 3: Prepare images ---
    image_paths = []
    downloader = None

    if args.image_urls:
        print(f"[pipeline] Step 3: Downloading {len(args.image_urls)} image(s)...")
        downloader = ImageDownloader(temp_dir=args.temp_dir)
        image_paths = downloader.download_all(args.image_urls)
        if not image_paths:
            print("Error: All image downloads failed.", file=sys.stderr)
            sys.exit(2)
    else:
        image_paths = args.images
        # Verify local files exist
        for p in image_paths:
            if not os.path.isfile(p):
                print(f"Error: Image file not found: {p}", file=sys.stderr)
                sys.exit(2)
        print(f"[pipeline] Step 3: Using {len(image_paths)} local image(s).")

    # --- Step 4: Fill form ---
    print("[pipeline] Step 4: Filling form...")
    try:
        publisher.publish(title=title, content=content, image_paths=image_paths)
        print("FILL_STATUS: READY_TO_PUBLISH")
    except CDPError as e:
        print(f"Error during form fill: {e}", file=sys.stderr)
        if downloader:
            downloader.cleanup()
        sys.exit(2)

    # --- Step 5: Publish (optional) ---
    if args.auto_publish:
        print("[pipeline] Step 5: Clicking publish button...")
        try:
            publisher._click_publish()
            print("PUBLISH_STATUS: PUBLISHED")
        except CDPError as e:
            print(f"Error clicking publish: {e}", file=sys.stderr)
            if downloader:
                downloader.cleanup()
            sys.exit(2)

    # --- Cleanup ---
    publisher.disconnect()
    if downloader:
        downloader.cleanup()

    print("[pipeline] Done.")


if __name__ == "__main__":
    main()
