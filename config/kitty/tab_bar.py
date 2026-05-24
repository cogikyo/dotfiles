# pyright: reportMissingImports=false
# pyright: reportCallIssue=false
# pyright: reportGeneralTypeIssues=false
# pyright: reportAttributeAccessIssue=false
"""Custom kitty tab bar via the draw_tab callback.

Layout: Icon → Tab titles (left) | CWD (child→root) → hostname (right).
Accent color follows the active agent; busy OpenCode tabs use manual vagari mode colors.
"""

import json
import os
import time
from getpass import getuser
from os import uname
from pathlib import Path
from unicodedata import east_asian_width

from kitty.boss import get_boss
from kitty.fast_data_types import Screen, get_options
from kitty.tab_bar import (
    DrawData,
    ExtraData,
    Formatter,
    TabBarData,
    as_rgb,
    draw_attributed_string,
    draw_title,
)
from kitty.utils import color_as_int

USER = getuser()
HOST = uname()[1]
LOCAL_HOST = HOST.split(".")[0]

# ╭──────────────────────────────────────────────────────────────────────────────╮
# │ Configuration                                                                │
# ╰──────────────────────────────────────────────────────────────────────────────╯

ICON_MAIN = "  "  # arch logo prefix
ICON_AGENT = " 󰯉 "  # agent icon prefix
AGENT_NAMES = ("opencode",)  # known AI agent CLIs

SEP_LEFT = ""  # left-pointing powerline arrow
SEP_RIGHT = ""  # right-pointing powerline arrow
SEP_SOFT = ""  # soft separator between same-bg tabs

ICON_HOME_DIR = " ⾕"  # home indicator
ICON_USER_TRUNCATED = " ⽙"  # shown when cwd is truncated

ICON_GIT_DIR = "  "  # if parent is git folder
ICON_GIT_OPENED = "  "  # git folder has multiple parts

ICON_ROOT_BASE = "  "  # root indicator when at root
ICON_ROOT_DESCENDED = " "  # root indicator when path has multiple parts

ICON_HOST = " ⾥"  # host indicator

# Layout
MAX_CWD_DEPTH = 6  # max directory levels to show in right status

# OpenCode writes this file from config/opencode/plugins/hyprd/kitty.ts.
KITTY_CONTEXT_PATH = (
    Path(os.environ["XDG_RUNTIME_DIR"]) / "opencode" / "kitty-context.json"
    if os.environ.get("XDG_RUNTIME_DIR")
    else Path("/tmp") / f"opencode-{os.getuid()}" / "kitty-context.json"
)
KITTY_CONTEXT_READ_TTL_SECONDS = 0.5
KITTY_CONTEXT_STALE_SECONDS = 24 * 60 * 60
OPENCODE_BUSY_CONTEXT_TTL_SECONDS = 1


# ╭──────────────────────────────────────────────────────────────────────────────╮
# │ Colors                                                                       │
# ╰──────────────────────────────────────────────────────────────────────────────╯


def _rgb(hex_value: str) -> int:
    return as_rgb(int(hex_value.removeprefix("#"), 16))


# Intentional manual coupling to vagari + OpenCode role semantics.
# Keep this semantic: the OpenCode context stores agent/status, not theme colors.
VAGARI_OPENCODE_MODE_HEX = {
    "primary": "#f8b486",
    "secondary": "#6380ec",
    "accent": "#b29ae8",
    "error": "#f08898",
    "warning": "#8aa4f3",
    "success": "#5fc976",
    "info": "#50dec8",
}

OPENCODE_AGENT_MODES = {
    "drive": "primary",
    "build": "secondary",
    "build.deep": "secondary",
    "build.fast": "secondary",
    "build.scribe": "secondary",
    "plan": "accent",
    "review": "error",
    "build": "secondary",
}

OPENCODE_AGENT_COLORS = {
    agent: _rgb(VAGARI_OPENCODE_MODE_HEX[mode])
    for agent, mode in OPENCODE_AGENT_MODES.items()
}
OPENCODE_AGENT_FALLBACK = _rgb(VAGARI_OPENCODE_MODE_HEX["primary"])


class Colors:
    """Color palette derived from kitty options."""

    def __init__(self):
        opts = get_options()
        self.fg = as_rgb(color_as_int(opts.background))
        self.bg = as_rgb(color_as_int(opts.color4))  # blue accent
        self.yellow = as_rgb(color_as_int(opts.color3))  # yellow/orange accent for opencode
        self.pink = as_rgb(color_as_int(opts.color13))  # pink accent for ssh sessions
        self.accent = as_rgb(color_as_int(opts.selection_background))
        self.active_bg = as_rgb(color_as_int(opts.active_tab_background))
        # Tab bar background (with fallback)
        bar_bg = opts.tab_bar_background if opts.tab_bar_background else 0
        self.bar_bg = as_rgb(color_as_int(bar_bg)) if bar_bg else 0


colors = Colors()


def get_accent() -> int:
    """Return accent color — OpenCode mode, pink for ssh, blue otherwise."""
    boss = get_boss()
    tm = boss.active_tab_manager if boss else None
    if tm:
        agent = _detect_active_agent(tm)
        if agent == "opencode":
            ctx_color = _opencode_color_for_active_window(tm)
            return ctx_color if ctx_color is not None else colors.yellow
        if _detect_ssh_active(tm)[1]:
            return colors.pink
    return colors.bg


# shared between draw_tab (writes) and draw_tab_title (reads) to reserve space
_right_status_length = 0
_current_icon_width = 0

# per-cycle cache — invalidated each time the first tab (index=1) is drawn
_cache: dict[str, object] = {}
_cache_cycle = -1

_kitty_context_cache: dict[str, object] = {
    "checked_at": 0.0,
    "mtime_ns": None,
    "contexts": (),
}

# ╭──────────────────────────────────────────────────────────────────────────────╮
# │ Utilities                                                                    │
# ╰──────────────────────────────────────────────────────────────────────────────╯


def _display_width(text: str) -> int:
    """Return terminal display width accounting for wide (CJK) characters."""
    return sum(2 if east_asian_width(c) in ("W", "F") else 1 for c in text)


def _as_int(value) -> int:
    try:
        return int(value)
    except (TypeError, ValueError):
        return 0


def _read_opencode_contexts() -> tuple:
    now = time.monotonic()
    if now - float(_kitty_context_cache["checked_at"]) < KITTY_CONTEXT_READ_TTL_SECONDS:
        return _kitty_context_cache["contexts"]

    _kitty_context_cache["checked_at"] = now
    try:
        stat = KITTY_CONTEXT_PATH.stat()
    except OSError:
        _kitty_context_cache["mtime_ns"] = None
        _kitty_context_cache["contexts"] = ()
        return ()

    if stat.st_mtime_ns == _kitty_context_cache["mtime_ns"]:
        return _kitty_context_cache["contexts"]

    try:
        data = json.loads(KITTY_CONTEXT_PATH.read_text())
    except (OSError, TypeError, ValueError, UnicodeDecodeError):
        contexts = ()
    else:
        contexts = _fresh_opencode_contexts(data)

    _kitty_context_cache["mtime_ns"] = stat.st_mtime_ns
    _kitty_context_cache["contexts"] = contexts
    return contexts


def _fresh_opencode_contexts(data) -> tuple:
    if not isinstance(data, dict):
        return ()

    now_ms = time.time() * 1000
    stale_ms = KITTY_CONTEXT_STALE_SECONDS * 1000
    contexts = []
    for ctx in data.values():
        if not isinstance(ctx, dict):
            continue
        updated_at = _as_int(ctx.get("updated_at"))
        if not updated_at or now_ms - updated_at > stale_ms:
            continue
        contexts.append(ctx)
    return tuple(contexts)


def _tab_id(tab) -> int:
    for attr in ("tab_id", "id"):
        value = _as_int(getattr(tab, attr, 0))
        if value:
            return value
    return 0


def _actual_tab(tab, index: int | None = None):
    boss = get_boss()
    tab_manager = boss.active_tab_manager if boss else None
    tabs = tuple(getattr(tab_manager, "tabs", ()) or ())
    tab_id = _tab_id(tab)

    if tab_id:
        for candidate in tabs:
            if _tab_id(candidate) == tab_id:
                return candidate

    if index is not None and 0 < index <= len(tabs):
        return tabs[index - 1]

    return tab


def _tab_windows(tab, index: int | None = None) -> tuple:
    actual = _actual_tab(tab, index)
    windows = getattr(actual, "windows", None)
    if windows is not None:
        try:
            return tuple(windows)
        except TypeError:
            return ()

    active_window = getattr(actual, "active_window", None)
    return (active_window,) if active_window else ()


def _kitty_window_id(window) -> int:
    # KITTY_WINDOW_ID is kitty's per-pty window id; window.id is the usual Python API surface.
    for attr in ("id", "window_id", "kitty_window_id"):
        value = _as_int(getattr(window, attr, 0))
        if value:
            return value
    return 0


def _opencode_agent_color(agent) -> int:
    name = str(agent or "").lower()
    if name in OPENCODE_AGENT_COLORS:
        return OPENCODE_AGENT_COLORS[name]
    return OPENCODE_AGENT_COLORS.get(name.split(".", 1)[0], OPENCODE_AGENT_FALLBACK)


def _opencode_busy_context_is_live(ctx) -> bool:
    updated_at = _as_int(ctx.get("updated_at"))
    return bool(updated_at and time.time() * 1000 - updated_at <= OPENCODE_BUSY_CONTEXT_TTL_SECONDS * 1000)


def _opencode_context_for_window_ids(window_ids: set[int]):
    for ctx in _read_opencode_contexts():
        kitty_window_id = _as_int(ctx.get("kitty_window_id"))
        if kitty_window_id and kitty_window_id in window_ids:
            return ctx
    return None


def _opencode_color_for_active_window(tab_manager) -> int | None:
    window = getattr(tab_manager, "active_window", None)
    window_id = _kitty_window_id(window) if window else 0
    if not window_id:
        return None

    ctx = _opencode_context_for_window_ids({window_id})
    agent = ctx.get("agent") if ctx else None
    return _opencode_agent_color(agent) if agent else None


def _opencode_context_for_tab(tab, index: int | None = None):
    windows = _tab_windows(tab, index)
    if not windows:
        return None

    window_ids = {_kitty_window_id(window) for window in windows}
    window_ids.discard(0)
    if not window_ids:
        return None

    return _opencode_context_for_window_ids(window_ids)


def _opencode_agent_fg_for_tab(tab, index: int | None = None) -> int | None:
    ctx = _opencode_context_for_tab(tab, index)
    if not ctx or not ctx.get("agent"):
        return None
    return _opencode_agent_color(ctx.get("agent"))


def _opencode_busy_fg_for_tab(tab, index: int | None = None) -> int | None:
    ctx = _opencode_context_for_tab(tab, index)
    if not ctx:
        return None
    if str(ctx.get("status", "")).lower() != "busy" or not _opencode_busy_context_is_live(ctx):
        return None
    return _opencode_agent_color(ctx.get("agent"))


def _tab_pill_colors(tab, base_bg: int, base_fg: int, index: int | None = None) -> tuple:
    return base_bg, _opencode_busy_fg_for_tab(tab, index) or _opencode_agent_fg_for_tab(tab, index) or base_fg


def _tab_pill_bg(tab, base_bg: int, index: int | None = None) -> int:
    return base_bg


def _is_git_repo(path: str) -> bool:
    """Check if path is inside a git repository (cached per draw cycle)."""
    cached = _cache.get("git_repo")
    if cached is not None:
        return cached
    p = Path(path)
    result = False
    while p != p.parent:
        if (p / ".git").exists():
            result = True
            break
        p = p.parent
    _cache["git_repo"] = result
    return result


def get_cwd_right() -> str:
    """Build CWD string in child→root order for right-aligned display (cached per draw cycle)."""
    cached = _cache.get("cwd_right")
    if cached is not None:
        return cached
    result = _compute_cwd_right()
    _cache["cwd_right"] = result
    return result


def _compute_cwd_right() -> str:
    """Compute the CWD string (uncached)."""
    boss = get_boss()
    tab_manager = boss.active_tab_manager
    if not tab_manager:
        return ""
    window = tab_manager.active_window
    if not window or not hasattr(window, "cwd_of_child"):
        return ""
    cwd = window.cwd_of_child
    if not cwd:
        return "/"

    parts = list(Path(cwd).parts)
    if not parts:
        return "/"

    is_git = _is_git_repo(cwd)

    # Determine root icon and extract directory parts
    if len(parts) > 1 and parts[1] in ("home", "Users"):
        dir_parts = parts[3:]  # skip /, home, username
        if is_git:
            root = ICON_GIT_DIR if len(dir_parts) <= 1 else ICON_GIT_OPENED
        else:
            root = ICON_HOME_DIR
    else:
        dir_parts = parts[1:]
        if is_git:
            root = ICON_GIT_DIR if len(dir_parts) <= 1 else ICON_GIT_OPENED
        else:
            root = ICON_ROOT_BASE if len(parts) == 1 else ICON_ROOT_DESCENDED

    if not dir_parts:
        return root

    # Limit depth, then reverse: child/parent/root
    if len(dir_parts) > MAX_CWD_DEPTH:
        dir_parts = dir_parts[-MAX_CWD_DEPTH:]
        return "/".join(reversed(dir_parts)) + ICON_USER_TRUNCATED

    return "/".join(reversed(dir_parts)) + root


# ╭──────────────────────────────────────────────────────────────────────────────╮
# │ Drawing Components                                                          │
# ╰──────────────────────────────────────────────────────────────────────────────╯


def _agent_from_window(window) -> str:
    """Return agent name if a known AI CLI is the foreground process."""
    try:
        for proc in window.child.foreground_processes:
            cmdline = proc.get("cmdline") or []
            if not cmdline:
                continue
            name = cmdline[0].rsplit("/", 1)[-1].lower()
            for agent in AGENT_NAMES:
                if agent in name:
                    return agent
    except (AttributeError, TypeError):
        pass
    return ""


def _agent_from_title(window) -> str:
    """Return agent name if the window title identifies a known agent."""
    title = getattr(window, "override_title", None) or getattr(window, "title", "")
    if not title:
        return ""
    title_lower = title.lower()
    for agent in AGENT_NAMES:
        if agent in title_lower:
            return agent
    return ""


def is_agent_window(tab_manager) -> bool:
    """Check tab/window titles for an agent without probing every child process."""
    cached = _cache.get("is_agent")
    if cached is not None:
        return cached
    result = False
    if not tab_manager:
        _cache["is_agent"] = False
        return False
    try:
        for tab in tab_manager.tabs:
            for window in tab.windows:
                if _agent_from_title(window):
                    result = True
                    break
            if result:
                break
    except (AttributeError, TypeError):
        pass
    _cache["is_agent"] = result
    return result


def _detect_active_agent(tab_manager) -> str:
    """Return the agent name running in the active window, or empty string (cached per draw cycle)."""
    cached = _cache.get("active_agent")
    if cached is not None:
        return cached
    if not tab_manager:
        result = ""
    else:
        window = tab_manager.active_window
        result = _agent_from_title(window) if window else ""
        if not result and window:
            result = _agent_from_window(window)
    _cache["active_agent"] = result
    return result


SSH_VALUE_OPTS = {
    "-b",
    "-c",
    "-D",
    "-E",
    "-e",
    "-F",
    "-I",
    "-i",
    "-J",
    "-L",
    "-l",
    "-m",
    "-O",
    "-o",
    "-p",
    "-Q",
    "-R",
    "-S",
    "-W",
    "-w",
}


def _parse_ssh_destination(cmdline: list) -> str:
    """Return the destination arg from an ssh cmdline, or empty string."""
    i = 1
    while i < len(cmdline):
        arg = cmdline[i]
        if arg.startswith("-") and arg != "-":
            # Attached value form like -p2222
            if len(arg) > 2 and arg[:2] in SSH_VALUE_OPTS:
                i += 1
                continue
            if arg in SSH_VALUE_OPTS:
                i += 2
                continue
            i += 1
            continue
        return arg
    return ""


def _ssh_from_window(window) -> tuple:
    """Return (user, host) when the active window is SSH'd to a *different* machine."""
    try:
        for proc in window.child.foreground_processes:
            cmdline = proc.get("cmdline") or []
            if not cmdline:
                continue
            name = cmdline[0].rsplit("/", 1)[-1].lower()
            if name != "ssh":
                continue
            dest = _parse_ssh_destination(cmdline)
            if not dest:
                continue
            if "@" in dest:
                user, host = dest.split("@", 1)
            else:
                user, host = "", dest
            host = host.split(".")[0]
            if host == LOCAL_HOST or host in ("localhost", "127.0.0.1", "::1"):
                continue
            return user, host
    except (AttributeError, TypeError):
        pass
    return "", ""


def _detect_ssh_active(tab_manager) -> tuple:
    """Return (user, host) for ssh in the active window, or ('', '') (cached per draw cycle)."""
    cached = _cache.get("active_ssh")
    if cached is not None:
        return cached
    if not tab_manager:
        result = ("", "")
    else:
        window = tab_manager.active_window
        result = _ssh_from_window(window) if window else ("", "")
    _cache["active_ssh"] = result
    return result


def draw_icon(screen: Screen, index: int) -> int:
    """Draw the main icon (only on first tab)."""
    global _current_icon_width
    if index != 1:
        return 0

    fg_prev, bg_prev = screen.cursor.fg, screen.cursor.bg

    boss = get_boss()
    tm = boss.active_tab_manager if boss else None
    agent = _detect_active_agent(tm) if tm else ""
    if tm and (agent or is_agent_window(tm)):
        icon = ICON_AGENT + (agent or "agents")
    else:
        icon = ICON_MAIN + USER

    accent = get_accent()

    # Icon with accent background
    screen.cursor.fg, screen.cursor.bg = colors.fg, accent
    screen.draw(icon)

    # Separator: icon → bar_bg
    screen.cursor.fg, screen.cursor.bg = accent, bg_prev
    screen.draw(SEP_LEFT)

    _current_icon_width = _display_width(icon + SEP_LEFT)
    screen.cursor.fg, screen.cursor.bg = fg_prev, bg_prev
    screen.cursor.x = _current_icon_width
    return screen.cursor.x


def draw_tab_title(
    draw_data: DrawData,
    screen: Screen,
    tab: TabBarData,
    index: int,
    extra_data: ExtraData,
    max_title_length: int,
) -> int:
    """Draw the tab title with appropriate separators."""
    global _right_status_length

    if screen.cursor.x >= screen.columns - _right_status_length:
        return screen.cursor.x

    base_fg = screen.cursor.fg
    tab_bg, tab_fg = _tab_pill_colors(tab, screen.cursor.bg, base_fg, index)
    screen.cursor.bg, screen.cursor.fg = tab_bg, tab_fg

    # Opening separator for first tab
    if index == 1:
        screen.cursor.fg, screen.cursor.bg = tab_bg, colors.bar_bg
        screen.draw(SEP_RIGHT)
        screen.cursor.bg, screen.cursor.fg = tab_bg, tab_fg

    default_bg = as_rgb(int(draw_data.default_bg))

    # Determine next tab background for separator style
    if extra_data.next_tab:
        next_tab_base_bg = as_rgb(draw_data.tab_bg(extra_data.next_tab))
        next_tab_bg = _tab_pill_bg(extra_data.next_tab, next_tab_base_bg)
        needs_soft_sep = next_tab_bg == tab_bg
    else:
        next_tab_bg = default_bg
        needs_soft_sep = False

    # Ensure cursor is past icon
    if screen.cursor.x <= _current_icon_width:
        screen.cursor.x = _current_icon_width

    # Draw tab content
    screen.draw(" ")
    screen.cursor.bg = tab_bg
    screen.cursor.fg = tab_fg
    title_draw_data = _draw_data_with_tab_fg(draw_data, tab_fg) if tab_fg != base_fg else draw_data
    draw_title(title_draw_data, screen, tab, index)

    # Draw appropriate separator
    if needs_soft_sep:
        _draw_soft_separator(screen, draw_data, tab_bg, tab_fg, default_bg)
    else:
        screen.draw(" ")
        screen.cursor.fg = tab_bg
        screen.cursor.bg = next_tab_bg
        screen.draw(SEP_LEFT)

    return screen.cursor.x


def _draw_data_with_tab_fg(draw_data: DrawData, fg: int) -> DrawData:
    if hasattr(draw_data, "_replace"):
        return draw_data._replace(active_fg=fg, inactive_fg=fg)
    return draw_data


def _draw_soft_separator(
    screen: Screen,
    draw_data: DrawData,
    tab_bg: int,
    tab_fg: int,
    default_bg: int,
) -> None:
    """Draw a soft separator between tabs with same background."""
    prev_fg = screen.cursor.fg

    if tab_bg == tab_fg:
        screen.cursor.fg = default_bg
    elif tab_bg != default_bg:
        c1 = draw_data.inactive_bg.contrast(draw_data.default_bg)
        c2 = draw_data.inactive_bg.contrast(draw_data.inactive_fg)
        if c1 < c2:
            screen.cursor.fg = default_bg

    screen.draw(" " + SEP_SOFT)
    screen.cursor.fg = prev_fg


def draw_right_status(screen: Screen, is_last: bool, cells: list) -> int:
    """Draw the right-aligned status (cwd + hostname)."""
    if not is_last:
        return screen.cursor.x

    draw_attributed_string(Formatter.reset, screen)
    screen.cursor.x = screen.columns - _right_status_length
    screen.cursor.fg = 0
    screen.cursor.bg = 0

    for fg, bg, text in cells:
        screen.cursor.fg = fg
        screen.cursor.bg = bg
        screen.draw(text)

    return screen.cursor.x


# ╭──────────────────────────────────────────────────────────────────────────────╮
# │ Main Entry Point                                                             │
# ╰──────────────────────────────────────────────────────────────────────────────╯


def draw_tab(
    draw_data: DrawData,
    screen: Screen,
    tab: TabBarData,
    before: int,
    max_title_length: int,
    index: int,
    is_last: bool,
    extra_data: ExtraData,
) -> int:
    """Kitty draw_tab callback — entry point for rendering each tab."""
    global _right_status_length, _cache_cycle

    # Invalidate per-cycle cache on first tab
    if index == 1:
        _cache_cycle += 1
        _cache.clear()

    # Build right status cells: separator | cwd | hostname
    cwd_text = " " + get_cwd_right() + " " + SEP_RIGHT
    boss = get_boss()
    tm = boss.active_tab_manager if boss else None
    ssh_user, ssh_host = _detect_ssh_active(tm) if tm else ("", "")
    if ssh_host:
        if ssh_user and ssh_user != USER:
            host_text = f"{ssh_user}@{ssh_host}{ICON_HOST}"
        else:
            host_text = f"{ssh_host}{ICON_HOST}"
    else:
        host_text = HOST + ICON_HOST
    accent = get_accent()

    cells = [
        (colors.active_bg, colors.bar_bg, SEP_RIGHT),
        (accent, colors.active_bg, cwd_text),
        (colors.fg, accent, host_text),
    ]

    # Calculate right status width
    _right_status_length = sum(_display_width(cell[2]) for cell in cells)

    # Draw components: Icon → Tabs → Right status
    draw_icon(screen, index)
    draw_tab_title(draw_data, screen, tab, index, extra_data, max_title_length)

    if is_last:
        draw_right_status(screen, is_last, cells)

    return screen.cursor.x
