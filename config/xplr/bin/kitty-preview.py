#!/usr/bin/env python3
import argparse
import array
import base64
import fcntl
import os
import sys
import tempfile
import termios
from pathlib import Path

PROTOCOL_START = b"\x1b_G"
PROTOCOL_END = b"\x1b\\"


def tty():
    return open("/dev/tty", "wb", buffering=0)


def send_graphics_command(out, keys, payload=b"", chunk_size=4096):
    if isinstance(payload, str):
        payload = payload.encode("utf-8")

    encoded = base64.standard_b64encode(payload)
    first = True
    for offset in range(0, max(1, len(encoded)), chunk_size):
        chunk = encoded[offset : offset + chunk_size]
        more = offset + chunk_size < len(encoded)
        command = b""
        if first:
            command = (",".join(f"{key}={value}" for key, value in keys.items()) + ",").encode("ascii")
            first = False
        command += b"m=1;" if more else b"m=0;"
        out.write(PROTOCOL_START + command + chunk + PROTOCOL_END)


def clear(out):
    send_graphics_command(out, {"a": "d", "d": "a", "q": 2})


def cell_size(out):
    buf = array.array("H", [0, 0, 0, 0])
    fcntl.ioctl(out.fileno(), termios.TIOCGWINSZ, buf)
    rows, cols, width_px, height_px = buf
    if cols == 0 or rows == 0 or width_px == 0 or height_px == 0:
        return 10, 20
    return max(width_px // cols, 1), max(height_px // rows, 1)


def fit_image(path, width_cells, height_cells, out):
	try:
		from PIL import Image
	except ImportError:
		return None

	source = Image.open(path)
	cell_width, cell_height = cell_size(out)
	max_size = (max(width_cells, 1) * cell_width, max(height_cells, 1) * cell_height)
	source.thumbnail(max_size, Image.Resampling.LANCZOS)
	used_width = max((source.width + cell_width - 1) // cell_width, 1)
	used_height = max((source.height + cell_height - 1) // cell_height, 1)
	return source, used_width, used_height


def load_image(path, image_id, width_cells, height_cells, out):
	fitted = fit_image(path, width_cells, height_cells, out)
	if fitted is None:
		return None
	source, used_width, used_height = fitted

	target = Path(tempfile.gettempdir()) / f"xplr-kitty-preview-{image_id}.png"
	source.save(target, format="PNG", compress_level=1)
	send_graphics_command(out, {"a": "t", "t": "f", "f": 100, "i": image_id, "q": 2}, str(target))
	return used_width, used_height


def display_image(image_id, x, y, out):
    out.write(f"\x1b[s\x1b[{y + 1};{x + 1}H".encode("ascii"))
    send_graphics_command(out, {"a": "p", "i": image_id, "q": 2})
    out.write(b"\x1b[u")


def display(path, image_id, x, y, width, height):
	with tty() as out:
		used = load_image(path, image_id, width, height, out)
		if used:
			used_width, used_height = used
			display_x = x + max((width - used_width) // 2, 0)
			display_y = y + max((height - used_height) // 2, 0)
			display_image(image_id, display_x, display_y, out)


def main():
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)

    subparsers.add_parser("clear")

    display_parser = subparsers.add_parser("display")
    display_parser.add_argument("path")
    display_parser.add_argument("image_id", type=int)
    display_parser.add_argument("x", type=int)
    display_parser.add_argument("y", type=int)
    display_parser.add_argument("width", type=int)
    display_parser.add_argument("height", type=int)

    args = parser.parse_args()
    if args.command == "clear":
        with tty() as out:
            clear(out)
    elif args.command == "display":
        display(args.path, args.image_id, args.x, args.y, args.width, args.height)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        sys.exit(1)
