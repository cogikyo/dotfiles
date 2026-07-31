"""Custom kitty kitten: choose a host, then move this OS window's managed tabs to it.

`main` runs in a kitty overlay window that owns a real PTY, so the built-in `ask`
kitten can draw its chooser on `/dev/tty`. A `launch --type=background` process has
no PTY, which is why the chooser cannot live on the hyprd side of this boundary.

Three contracts hold this together:

- `ask` prints its answer as JSON on stdout while our own stdout carries the
  custom-kitten result protocol, so the child's stdout and stderr are captured.
- `handle_result` runs inside the kitty process, so it is the only place that can
  read the kitty PID and the immutable OS-window id that hyprd needs.
- hyprd rejects a window whose panes do not match a tab profile exactly, so the
  dispatch waits until the chooser overlay has left the window group.
"""

import json
import os
import shutil
import subprocess

HOSTS = {"a": "abbott", "c": "costello", "n": "neumann"}
QUIT = "q"

# Enter selects QUIT, so an accidental confirm never forwards a host switch.
ASK_DEFAULT = QUIT

OVERLAY_POLL_INTERVAL = 0.02
OVERLAY_POLL_LIMIT = 250


def main(args):
    from kitty.constants import kitten_exe

    command = [
        kitten_exe(),
        "ask",
        "--type=choices",
        "--title=Switch host",
        "--message=Move this kitty OS window to host:",
        f"--default={ASK_DEFAULT}",
    ]
    command += [f"--choice={key}:{alias}" for key, alias in HOSTS.items()]
    command.append(f"--choice={QUIT}:quit")

    env = dict(os.environ)
    # A kitten that believes it is the UI writes the result escape onto stdout,
    # and stdout here belongs to this kitten's own result.
    env.pop("KITTEN_RUNNING_AS_UI", None)

    try:
        chooser = subprocess.run(
            command, capture_output=True, text=True, env=env, check=False
        )
    except OSError as err:
        return {"error": f"cannot run the ask kitten: {err}"}

    if chooser.returncode != 0:
        detail = first_line(chooser.stderr)
        if not detail:
            # Interrupted without a message: treat as a cancel.
            return None
        return {"error": f"ask kitten failed (exit {chooser.returncode}): {detail}"}

    try:
        answer = json.loads(chooser.stdout or "{}")
    except ValueError:
        return {
            "error": f"unreadable ask kitten answer: {first_line(chooser.stdout)!r}"
        }
    if not isinstance(answer, dict):
        return {"error": "unreadable ask kitten answer"}

    # Esc answers with an empty response; Enter answers with ASK_DEFAULT.
    choice = str(answer.get("response") or "").strip()
    if choice in ("", QUIT, "quit"):
        return None
    alias = HOSTS.get(choice) or (choice if choice in HOSTS.values() else "")
    if not alias:
        return {"error": f"unexpected chooser answer {choice!r}"}
    return {"host": alias}


def handle_result(args, answer, target_window_id, boss):
    if not isinstance(answer, dict):
        return
    error = answer.get("error")
    if error:
        boss.show_error("Host switch failed", error)
        return
    alias = answer.get("host")
    if alias not in HOSTS.values():
        return

    window = boss.window_id_map.get(target_window_id)
    if window is None:
        boss.show_error("Host switch failed", "The originating kitty window is gone")
        return

    hyprd = shutil.which("hyprd")
    if not hyprd:
        boss.show_error("Host switch failed", "hyprd is not on kitty's PATH")
        return

    dispatch_after_overlay_closes(boss, window, alias, hyprd)


def dispatch_after_overlay_closes(boss, window, alias, hyprd):
    """Run hyprd once the chooser overlay has left the originating window group.

    The kitty PID and the OS-window id are captured now and never re-resolved, so a
    focus change between the chooser and the switch cannot retarget another window.
    """
    from kitty.fast_data_types import add_timer

    window_id = window.id
    command = [
        hyprd,
        "tabs",
        "host",
        alias,
        "--kitty-pid",
        str(os.getpid()),
        "--os-window",
        str(window.os_window_id),
    ]
    polls = 0

    def dispatch(timer_id):
        nonlocal polls
        live = boss.window_id_map.get(window_id)
        if live is None:
            boss.show_error("Host switch failed", "The originating kitty window closed")
            return
        if overlay_present(live):
            polls += 1
            if polls > OVERLAY_POLL_LIMIT:
                boss.show_error(
                    "Host switch failed", "The host chooser overlay did not close"
                )
                return
            add_timer(dispatch, OVERLAY_POLL_INTERVAL, False)
            return
        run_hyprd(boss, command, alias)

    add_timer(dispatch, 0, False)


def overlay_present(window):
    """True while an overlay still shares the window's group, e.g. the chooser."""
    tab = window.tabref()
    if tab is None:
        return False
    group = tab.windows.group_for_window(window)
    return group is not None and len(group) > 1


def run_hyprd(boss, command, alias):
    """Dispatch through kitty and surface CLI or transport failures in a kitty window.

    hyprd reports daemon-side failures itself through notifications; this pipe only
    exists so a failure to reach the daemon is not swallowed.
    """
    read_fd, write_fd = os.pipe()
    os.set_blocking(read_fd, False)

    def finished(exit_status, err):
        output = drain(read_fd)
        for fd in (read_fd, write_fd):
            try:
                os.close(fd)
            except OSError:
                pass
        if err is not None:
            boss.show_error("Host switch failed", f"Could not run hyprd: {err}")
            return
        if exit_status:
            detail = first_line(output) or f"exit status {exit_status}"
            boss.show_error("Host switch failed", f"hyprd tabs host {alias}: {detail}")

    boss.run_background_process(
        command, stdout=write_fd, stderr=write_fd, notify_on_death=finished
    )


def drain(fd):
    chunks = []
    while True:
        try:
            chunk = os.read(fd, 4096)
        except OSError:
            break
        if not chunk:
            break
        chunks.append(chunk)
    return b"".join(chunks).decode("utf-8", "replace")


def first_line(text):
    for line in (text or "").splitlines():
        line = line.strip()
        if line:
            return line
    return ""
