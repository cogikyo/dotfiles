# /// script
# dependencies = ["pyte"]
# ///
"""Capture an OpenCode TUI screen for a session through a pseudo-terminal.

Usage, from config/opencode:
    uv run plugins/capture.py SESSION [--dir DIR] [--seconds N] [--keys TEXT] [--size COLSxROWS] [--out FILE]

--keys is typed once at the halfway point; backslash escapes such as \\r (Enter), \\x0f (ctrl+o), and \\x1b (esc) are decoded.
"""

import argparse
import fcntl
import os
import pty
import select
import signal
import struct
import sys
import termios
import time

import pyte


def size(text: str) -> tuple[int, int]:
    cols, _, rows = text.lower().partition("x")
    try:
        return int(cols), int(rows)
    except ValueError:
        raise argparse.ArgumentTypeError(f"expected COLSxROWS, got {text!r}")


def escapes(text: str) -> bytes:
    return text.encode("latin-1", "backslashreplace").decode("unicode_escape").encode()


def stop(pid: int) -> None:
    for sig, grace in ((signal.SIGTERM, 2.0), (signal.SIGKILL, 2.0)):
        try:
            os.killpg(pid, sig)
        except ProcessLookupError:
            return
        deadline = time.monotonic() + grace
        while time.monotonic() < deadline:
            done, _ = os.waitpid(pid, os.WNOHANG)
            if done:
                return
            time.sleep(0.05)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Capture an OpenCode TUI screen through a pseudo-terminal."
    )
    parser.add_argument("session")
    parser.add_argument("--dir", default=os.getcwd())
    parser.add_argument("--seconds", type=float, default=12)
    parser.add_argument("--keys")
    parser.add_argument("--size", type=size, default=(150, 60))
    parser.add_argument("--out")
    args = parser.parse_args()
    cols, rows = args.size

    screen = pyte.Screen(cols, rows)
    stream = pyte.ByteStream(screen)
    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(args.dir)
        env = dict(
            os.environ, TERM="xterm-256color", COLUMNS=str(cols), LINES=str(rows)
        )
        os.execvpe("opencode", ["opencode", "-s", args.session, args.dir], env)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))

    keys = escapes(args.keys) if args.keys else None
    start = time.monotonic()
    end = start + args.seconds
    received = 0
    early = False
    while time.monotonic() < end:
        if keys and time.monotonic() >= start + args.seconds / 2:
            os.write(fd, keys)
            keys = None
        ready, _, _ = select.select([fd], [], [], 0.2)
        if fd not in ready:
            continue
        try:
            data = os.read(fd, 65536)
        except OSError:
            data = b""
        if not data:
            early = True
            break
        received += len(data)
        stream.feed(data)
        if b"\x1b[c" in data or b"\x1b[>c" in data:
            os.write(fd, b"\x1b[?62;22c")
    stop(pid)
    os.close(fd)

    lines = [line.rstrip() for line in screen.display]
    while lines and not lines[-1]:
        lines.pop()
    text = "".join(f"{line}\n" for line in lines)
    if args.out:
        with open(args.out, "w") as handle:
            handle.write(text)
    else:
        sys.stdout.write(text)
    if early and received == 0:
        print("capture: opencode exited before drawing anything", file=sys.stderr)
        return 1
    if early:
        print(
            "capture: opencode exited before the capture window ended", file=sys.stderr
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())
