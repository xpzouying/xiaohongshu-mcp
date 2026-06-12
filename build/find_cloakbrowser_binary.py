#!/usr/bin/env python3
"""Prepare CloakBrowser Chromium for Docker images."""

import argparse
import os
import shutil
from pathlib import Path

from cloakbrowser import ensure_binary


def disable_nonessential_build_hooks() -> None:
    """Disable CloakBrowser side effects that are not needed while building images."""
    # Docker builds only need the binary. CloakBrowser's welcome output and
    # background update check have caused ARM64/QEMU build exits to segfault.
    globals_ = ensure_binary.__globals__
    globals_["_show_welcome"] = lambda: None
    globals_["_maybe_trigger_update_check"] = lambda: None


def find_binary() -> Path:
    disable_nonessential_build_hooks()
    binary = Path(ensure_binary()).expanduser()
    if not binary.is_file() or not os.access(binary, os.X_OK):
        raise SystemExit(f"CloakBrowser Chromium binary is not executable: {binary}")

    return binary


def copy_to_install_home(binary: Path, install_home: Path) -> Path:
    source_home = Path.home()
    source_cache = source_home / ".cloakbrowser"
    if not source_cache.is_dir():
        raise SystemExit(f"CloakBrowser cache directory not found: {source_cache}")

    try:
        relative_binary = binary.relative_to(source_home)
    except ValueError as exc:
        raise SystemExit(f"CloakBrowser binary is outside HOME: {binary}") from exc

    install_home.mkdir(parents=True, exist_ok=True)
    target_cache = install_home / ".cloakbrowser"
    if target_cache.exists():
        shutil.rmtree(target_cache)
    shutil.copytree(source_cache, target_cache, symlinks=True)

    installed_binary = install_home / relative_binary
    if not installed_binary.is_file() or not os.access(installed_binary, os.X_OK):
        raise SystemExit(f"Installed CloakBrowser binary is not executable: {installed_binary}")

    return installed_binary


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--install-home", help="Copy CloakBrowser cache into this HOME")
    parser.add_argument("--link", help="Create or replace a symlink to the binary")
    args = parser.parse_args()

    binary = find_binary()
    if args.install_home:
        binary = copy_to_install_home(binary, Path(args.install_home))

    print(binary)

    if args.link:
        link = Path(args.link)
        if link.exists() or link.is_symlink():
            link.unlink()
        link.symlink_to(binary)


if __name__ == "__main__":
    main()
