"""
Multi-account manager for Xiaohongshu publishing.

Manages multiple Xiaohongshu accounts with separate Chrome profiles:
- Each account has its own user-data-dir for cookie isolation
- Accounts are stored in a JSON config file
- Supports add/remove/list/switch operations

Usage:
    python account_manager.py list
    python account_manager.py add <name> [--alias <alias>]
    python account_manager.py remove <name>
    python account_manager.py info <name>
    python account_manager.py set-default <name>
"""

import json
import os
import sys
import shutil
from typing import Optional

# Config file location
CONFIG_DIR = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "config")
ACCOUNTS_FILE = os.path.join(CONFIG_DIR, "accounts.json")

# Base directory for account profiles
PROFILES_BASE = os.path.join(os.environ.get("LOCALAPPDATA", os.path.expanduser("~")),
                              "Google", "Chrome", "XiaohongshuProfiles")

# Default account name (for backward compatibility)
DEFAULT_PROFILE_NAME = "default"


def _ensure_config_dir():
    """Ensure the config directory exists."""
    os.makedirs(CONFIG_DIR, exist_ok=True)


def _load_accounts() -> dict:
    """Load accounts from config file."""
    _ensure_config_dir()
    if os.path.exists(ACCOUNTS_FILE):
        try:
            with open(ACCOUNTS_FILE, "r", encoding="utf-8") as f:
                return json.load(f)
        except (json.JSONDecodeError, IOError):
            pass
    # Default structure
    return {
        "default_account": DEFAULT_PROFILE_NAME,
        "accounts": {
            DEFAULT_PROFILE_NAME: {
                "alias": "默认账号",
                "profile_dir": os.path.join(PROFILES_BASE, DEFAULT_PROFILE_NAME),
                "created_at": None,
            }
        }
    }


def _save_accounts(data: dict):
    """Save accounts to config file."""
    _ensure_config_dir()
    with open(ACCOUNTS_FILE, "w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)


def get_profile_dir(account_name: Optional[str] = None) -> str:
    """
    Get the Chrome profile directory for a given account.

    Args:
        account_name: Account name. If None, uses the default account.

    Returns:
        Path to the Chrome user-data-dir for this account.
    """
    data = _load_accounts()

    if account_name is None:
        account_name = data.get("default_account", DEFAULT_PROFILE_NAME)

    if account_name not in data["accounts"]:
        # Fallback to default
        account_name = DEFAULT_PROFILE_NAME
        if account_name not in data["accounts"]:
            # Create default account entry
            data["accounts"][account_name] = {
                "alias": "默认账号",
                "profile_dir": os.path.join(PROFILES_BASE, account_name),
                "created_at": None,
            }
            _save_accounts(data)

    return data["accounts"][account_name]["profile_dir"]


def get_default_account() -> str:
    """Get the name of the default account."""
    data = _load_accounts()
    return data.get("default_account", DEFAULT_PROFILE_NAME)


def set_default_account(account_name: str) -> bool:
    """
    Set the default account.

    Returns True if successful, False if account doesn't exist.
    """
    data = _load_accounts()
    if account_name not in data["accounts"]:
        return False
    data["default_account"] = account_name
    _save_accounts(data)
    return True


def list_accounts() -> list[dict]:
    """
    List all registered accounts.

    Returns a list of dicts with account info.
    """
    data = _load_accounts()
    default = data.get("default_account", DEFAULT_PROFILE_NAME)
    result = []
    for name, info in data["accounts"].items():
        result.append({
            "name": name,
            "alias": info.get("alias", ""),
            "profile_dir": info.get("profile_dir", ""),
            "is_default": name == default,
        })
    return result


def add_account(name: str, alias: Optional[str] = None) -> bool:
    """
    Add a new account.

    Args:
        name: Unique account name (used as identifier)
        alias: Display name / description

    Returns True if added, False if name already exists.
    """
    data = _load_accounts()
    if name in data["accounts"]:
        return False

    from datetime import datetime
    profile_dir = os.path.join(PROFILES_BASE, name)
    os.makedirs(profile_dir, exist_ok=True)

    data["accounts"][name] = {
        "alias": alias or name,
        "profile_dir": profile_dir,
        "created_at": datetime.now().isoformat(),
    }
    _save_accounts(data)
    return True


def remove_account(name: str, delete_profile: bool = False) -> bool:
    """
    Remove an account.

    Args:
        name: Account name to remove
        delete_profile: If True, also delete the Chrome profile directory

    Returns True if removed, False if not found or is default.
    """
    data = _load_accounts()
    if name not in data["accounts"]:
        return False

    # Don't allow removing the default account if it's the only one
    if name == data.get("default_account") and len(data["accounts"]) == 1:
        return False

    profile_dir = data["accounts"][name].get("profile_dir", "")
    del data["accounts"][name]

    # If we removed the default, set a new default
    if name == data.get("default_account"):
        data["default_account"] = next(iter(data["accounts"].keys()))

    _save_accounts(data)

    # Optionally delete the profile directory
    if delete_profile and profile_dir and os.path.isdir(profile_dir):
        try:
            shutil.rmtree(profile_dir)
        except Exception:
            pass

    return True


def get_account_info(name: str) -> Optional[dict]:
    """Get info for a specific account."""
    data = _load_accounts()
    if name not in data["accounts"]:
        return None
    info = data["accounts"][name].copy()
    info["name"] = name
    info["is_default"] = name == data.get("default_account")
    return info


def account_exists(name: str) -> bool:
    """Check if an account exists."""
    data = _load_accounts()
    return name in data["accounts"]


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main():
    import argparse

    parser = argparse.ArgumentParser(description="Xiaohongshu Account Manager")
    sub = parser.add_subparsers(dest="command", required=True)

    # list
    sub.add_parser("list", help="List all accounts")

    # add
    p_add = sub.add_parser("add", help="Add a new account")
    p_add.add_argument("name", help="Account name (unique identifier)")
    p_add.add_argument("--alias", help="Display name / description")

    # remove
    p_rm = sub.add_parser("remove", help="Remove an account")
    p_rm.add_argument("name", help="Account name to remove")
    p_rm.add_argument("--delete-profile", action="store_true",
                      help="Also delete the Chrome profile directory")

    # info
    p_info = sub.add_parser("info", help="Show account info")
    p_info.add_argument("name", help="Account name")

    # set-default
    p_def = sub.add_parser("set-default", help="Set the default account")
    p_def.add_argument("name", help="Account name to set as default")

    # get-profile-dir (for internal use)
    p_dir = sub.add_parser("get-profile-dir", help="Get profile directory for an account")
    p_dir.add_argument("--account", help="Account name (default: default account)")

    args = parser.parse_args()

    if args.command == "list":
        accounts = list_accounts()
        if not accounts:
            print("No accounts configured.")
            return
        print(f"{'Name':<20} {'Alias':<20} {'Default':<10}")
        print("-" * 50)
        for acc in accounts:
            default_mark = "*" if acc["is_default"] else ""
            print(f"{acc['name']:<20} {acc['alias']:<20} {default_mark:<10}")

    elif args.command == "add":
        if add_account(args.name, args.alias):
            print(f"Account '{args.name}' added.")
            print(f"Profile dir: {get_profile_dir(args.name)}")
            print("\nTo log in to this account, run:")
            print(f"  python cdp_publish.py --account {args.name} login")
        else:
            print(f"Error: Account '{args.name}' already exists.", file=sys.stderr)
            sys.exit(1)

    elif args.command == "remove":
        if remove_account(args.name, args.delete_profile):
            print(f"Account '{args.name}' removed.")
        else:
            print(f"Error: Cannot remove account '{args.name}'.", file=sys.stderr)
            sys.exit(1)

    elif args.command == "info":
        info = get_account_info(args.name)
        if info:
            print(f"Name: {info['name']}")
            print(f"Alias: {info.get('alias', '')}")
            print(f"Profile dir: {info.get('profile_dir', '')}")
            print(f"Default: {'Yes' if info.get('is_default') else 'No'}")
            print(f"Created: {info.get('created_at', 'Unknown')}")
        else:
            print(f"Error: Account '{args.name}' not found.", file=sys.stderr)
            sys.exit(1)

    elif args.command == "set-default":
        if set_default_account(args.name):
            print(f"Default account set to '{args.name}'.")
        else:
            print(f"Error: Account '{args.name}' not found.", file=sys.stderr)
            sys.exit(1)

    elif args.command == "get-profile-dir":
        print(get_profile_dir(args.account))


if __name__ == "__main__":
    main()
