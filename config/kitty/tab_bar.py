# pyright: reportMissingImports=false
# pyright: reportCallIssue=false
# pyright: reportGeneralTypeIssues=false
# pyright: reportAttributeAccessIssue=false
"""
Custom Kitty tab bar with:
  - Left: Icon → CWD → Tab titles
  - Right: user@hostname status
"""

from os import getlogin, uname
from pathlib import Path

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

# ╭──────────────────────────────────────────────────────────────────────────────╮
# │ Configuration                                                                │
# ╰──────────────────────────────────────────────────────────────────────────────╯

ICON_MAIN = "  "  # arch logo
ICON_CLAUDE = " 󰯉 "  # claude icon

SEP_LEFT = ""  # left-pointing powerline arrow
SEP_RIGHT = ""  # right-pointing powerline arrow
SEP_SOFT = ""  # soft separator between same-bg tabs

TRUNCATE = " ⽙"  # shown when cwd is truncated
ICON_HOME = " ⾕"  # home indicator
CWD_SPACER = " 󰓩 "  # decorative spacer after cwd

ICON_ROOT_DESCENDED = "  "  # root indicator when path has multiple parts
ICON_ROOT_BASE = "  "  # root indicator when at root

ICON_USER = "⼈"  # user indicator
ICON_HOST = " ⾥"  # host indicator

# Layout
RIGHT_MARGIN = -8
MAX_CWD_DEPTH = 2  # max directory levels to show


# ╭──────────────────────────────────────────────────────────────────────────────╮
# │ Colors                                                                       │
# ╰──────────────────────────────────────────────────────────────────────────────╯


class Colors:
    """Color palette derived from kitty options."""

    def __init__(self):
        opts = get_options()
        self.fg = as_rgb(color_as_int(opts.background))
        self.bg = as_rgb(color_as_int(opts.color4))  # blue accent
        self.red = as_rgb(color_as_int(opts.color1))  # red accent for claude
        self.accent = as_rgb(color_as_int(opts.selection_background))
        self.active_bg = as_rgb(color_as_int(opts.active_tab_background))

        # Tab bar background (with fallback)
        bar_bg = opts.tab_bar_background if opts.tab_bar_background else 0
        self.bar_bg = as_rgb(color_as_int(bar_bg)) if bar_bg else 0


colors = Colors()


def get_accent() -> int:
    """Get the accent color based on window type (red for Claude, blue otherwise)."""
    boss = get_boss()
    if boss and is_claude_window(boss.active_tab_manager):
        return colors.red
    return colors.bg


_right_status_length = 0

# ╭──────────────────────────────────────────────────────────────────────────────╮
# │ Utilities                                                                    │
# ╰──────────────────────────────────────────────────────────────────────────────╯


def get_cwd() -> str:
    """Get formatted current working directory from active window."""
    boss = get_boss()
    tab_manager = boss.active_tab_manager
    if not tab_manager:
        return ""
    window = tab_manager.active_window
    if not window or not hasattr(window, "cwd_of_child"):
        return ""
    cwd = window.cwd_of_child

    if not cwd:
        return ICON_ROOT_BASE

    parts = list(Path(cwd).parts)

    if not parts:
        return ICON_ROOT_BASE

    # Replace /home/user or /Users/user with home icon
    if len(parts) > 1 and parts[1] in ("home", "Users"):
        parts[0] = ICON_HOME
        parts[1:3] = []
    else:
        # Use different root icon based on path depth
        if len(parts) > 1:
            parts[0] = ICON_ROOT_DESCENDED
        else:
            parts[0] = ICON_ROOT_BASE

    # Limit to MAX_CWD_DEPTH directories (excluding the icon prefix)
    dir_parts = parts[1:]  # directories without the icon
    if len(dir_parts) > MAX_CWD_DEPTH:
        # Show truncation symbol + last MAX_CWD_DEPTH directories
        return TRUNCATE + "/".join(dir_parts[-MAX_CWD_DEPTH:])

    return parts[0] + "/".join(dir_parts)


# ╭─────────────────────────────────────────────────────────────────────────────╮
# │ Drawing Components                                                          │
# ╰─────────────────────────────────────────────────────────────────────────────╯


def get_window_title() -> str:
    """Get the active window's title."""
    boss = get_boss()
    if not boss:
        return ""
    tm = boss.active_tab_manager
    if not tm:
        return ""
    window = tm.active_window
    if not window:
        return ""
    return window.title or ""


def is_claude_window(tab_manager) -> bool:
    """Check if any window in the tab manager was launched as a Claude window.

    This checks the initial title set when the window was created
    (e.g., via `kitty --title claude`), not the current active title.
    """
    if not tab_manager:
        return False
    try:
        # Check all tabs and their windows
        for tab in tab_manager.tabs:
            for window in tab.windows:
                # Try override_title first (set by --title flag), then title
                title = getattr(window, "override_title", None) or getattr(
                    window, "title", ""
                )
                if title and "claude" in title.lower():
                    return True
    except (AttributeError, TypeError):
        pass
    return False


def draw_icon(screen: Screen, index: int) -> int:
    """Draw the main icon (only on first tab)."""
    if index != 1:
        return 0

    fg_prev, bg_prev = screen.cursor.fg, screen.cursor.bg

    # Choose icon based on initial window title (e.g., kitty launched with --title claude)
    icon = ICON_MAIN
    boss = get_boss()
    if boss:
        tm = boss.active_tab_manager
        if is_claude_window(tm):
            icon = ICON_CLAUDE

    accent = get_accent()

    # Icon with accent background
    screen.cursor.fg, screen.cursor.bg = colors.fg, accent
    screen.draw(icon)

    # Separator
    screen.cursor.fg, screen.cursor.bg = accent, bg_prev
    screen.draw(SEP_LEFT)

    screen.cursor.fg, screen.cursor.bg = fg_prev, bg_prev
    screen.cursor.x = len(ICON_MAIN + SEP_LEFT)
    return screen.cursor.x


def draw_cwd(screen: Screen, index: int) -> int:
    """Draw the current working directory (only at index 1, shows active tab's cwd)."""
    if index != 1:
        return 0

    fg_prev, bg_prev = screen.cursor.fg, screen.cursor.bg
    cwd = get_cwd()  # Get active tab's CWD

    # CWD text
    screen.cursor.fg, screen.cursor.bg = get_accent(), colors.active_bg
    screen.draw(cwd)

    # Separator + spacer
    screen.cursor.fg, screen.cursor.bg = colors.active_bg, colors.bar_bg
    screen.draw(SEP_LEFT)
    screen.draw(CWD_SPACER)

    screen.cursor.fg, screen.cursor.bg = fg_prev, bg_prev
    screen.cursor.x = len(cwd) + 9
    return screen.cursor.x


def draw_tab_title(
    draw_data: DrawData,
    screen: Screen,
    tab: TabBarData,
    index: int,
    extra_data: ExtraData,
) -> int:
    """Draw the tab title with appropriate separators."""
    global _right_status_length

    if screen.cursor.x >= screen.columns - _right_status_length:
        return screen.cursor.x

    tab_bg, tab_fg = screen.cursor.bg, screen.cursor.fg

    # Opening separator for first tab
    if index == 1:
        screen.cursor.fg, screen.cursor.bg = tab_bg, colors.bar_bg
        screen.draw(SEP_RIGHT)
        screen.cursor.bg = tab_bg

    default_bg = as_rgb(int(draw_data.default_bg))

    # Determine next tab background for separator style
    if extra_data.next_tab:
        next_tab_bg = as_rgb(draw_data.tab_bg(extra_data.next_tab))
        needs_soft_sep = next_tab_bg == tab_bg
    else:
        next_tab_bg = default_bg
        needs_soft_sep = False

    # Ensure cursor is past icon
    if screen.cursor.x <= len(ICON_MAIN):
        screen.cursor.x = len(ICON_MAIN)

    # Draw tab content
    screen.draw(" ")
    screen.cursor.bg = tab_bg
    draw_title(draw_data, screen, tab, index)

    # Draw appropriate separator
    if needs_soft_sep:
        _draw_soft_separator(screen, draw_data, tab_bg, tab_fg, default_bg)
    else:
        screen.draw(" ")
        screen.cursor.fg = tab_bg
        screen.cursor.bg = next_tab_bg
        screen.draw(SEP_LEFT)

    return screen.cursor.x


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
    """Draw the right-aligned status (user@host)."""
    if not is_last:
        screen.cursor.bg = colors.fg
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
    """Main entry point for drawing each tab."""
    global _right_status_length

    # Build right status cells: separator | user | hostname
    user_text = ICON_USER + getlogin() + " " + SEP_RIGHT
    host_text = uname()[1] + ICON_HOST
    accent = get_accent()

    cells = [
        (colors.active_bg, colors.bar_bg, SEP_RIGHT),
        (accent, colors.active_bg, user_text),
        (colors.fg, accent, host_text),
    ]

    # Calculate right status width
    _right_status_length = RIGHT_MARGIN + 2
    for cell in cells:
        _right_status_length += len(str(cell[1]))

    # Draw components
    draw_icon(screen, index)
    draw_cwd(screen, index)
    draw_tab_title(draw_data, screen, tab, index, extra_data)
    draw_right_status(screen, is_last, cells)

    return screen.cursor.x
