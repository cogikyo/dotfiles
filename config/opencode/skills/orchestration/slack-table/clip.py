#!/usr/bin/env python3
import array
import html
import os
import socket
import struct
import sys

text = sys.stdin.read().replace("\r", "").strip("\n")
if not text:
    sys.exit("clip.py: empty table on stdin")
rows = [r.split("\t") for r in text.split("\n")]

# Mirrors LeadPier frontend packages/shared/src/components/Table/copy/serialize.ts.
CELL = "border:1px solid #d1d5db;padding:2px 16px 2px 4px;text-align:left;white-space:nowrap;"


def tr(cells, tag):
    scope = ' scope="col"' if tag == "th" else ""
    return (
        "<tr>"
        + "".join(
            f'<{tag}{scope} style="{CELL}">{html.escape(c)}</{tag}>' for c in cells
        )
        + "</tr>"
    )


page = (
    '<meta http-equiv="content-type" content="text/html; charset=utf-8">'
    '<table style="border-collapse:collapse;">'
    f"<thead>{tr(rows[0], 'th')}</thead><tbody>{''.join(tr(r, 'td') for r in rows[1:])}</tbody></table>"
).encode()
plain = "\n".join("\t".join(r) for r in rows).encode()
offers = {
    "text/html": page,
    "text/plain;charset=utf-8": plain,
    "text/plain": plain,
    "UTF8_STRING": plain,
    "STRING": plain,
    "TEXT": plain,
}

sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
try:
    sock.connect(
        os.path.join(
            os.environ["XDG_RUNTIME_DIR"],
            os.environ.get("WAYLAND_DISPLAY", "wayland-0"),
        )
    )
except (KeyError, OSError) as e:
    sys.exit(f"clip.py: cannot reach the Wayland display: {e}")


def wstr(v):
    b = v.encode() + b"\0"
    return struct.pack("<I", len(b)) + b + b"\0" * (-len(b) % 4)


def rstr(body, off):
    (n,) = struct.unpack_from("<I", body, off)
    return body[off + 4 : off + 3 + n].decode(), off + 4 + n + (-n % 4)


def request(obj, op, payload=b""):
    sock.sendall(struct.pack("<II", obj, (8 + len(payload)) << 16 | op) + payload)


fds = []


def events():
    buf = b""
    while True:
        while len(buf) >= 8:
            obj, word = struct.unpack_from("<II", buf)
            size = word >> 16
            if len(buf) < size:
                break
            body, buf = buf[8:size], buf[size:]
            if obj == 1 and word & 0xFFFF == 0:
                sys.exit(f"clip.py: wayland error: {rstr(body, 8)[0]}")
            yield obj, word & 0xFFFF, body
        msg, anc, _, _ = sock.recvmsg(65536, socket.CMSG_SPACE(28 * 4))
        if not msg:
            sys.exit("clip.py: compositor closed the connection")
        for level, kind, data in anc:
            if level == socket.SOL_SOCKET and kind == socket.SCM_RIGHTS:
                a = array.array("i")
                a.frombytes(data[: len(data) - len(data) % 4])
                fds.extend(a)
        buf += msg


REGISTRY, SYNC, MANAGER, SEAT, SOURCE, DEVICE, SYNC2 = 2, 3, 4, 5, 6, 7, 8


def serve(obj, op, body):
    """Answer a paste request; exit when another client takes the selection."""
    if obj != SOURCE:
        return
    if op == 1:
        sys.exit(
            "clip.py: selection was replaced before it was confirmed"
            if os.getpid() == parent
            else 0
        )
    mime, _ = rstr(body, 0)
    fd = fds.pop(0)
    try:
        data = memoryview(offers.get(mime, plain))
        while data:
            data = data[os.write(fd, data) :]
    except BrokenPipeError:
        pass
    finally:
        os.close(fd)


parent = os.getpid()
stream = events()
request(1, 1, struct.pack("<I", REGISTRY))
request(1, 0, struct.pack("<I", SYNC))
found = {}
for obj, op, body in stream:
    if obj == REGISTRY and op == 0:
        (name,) = struct.unpack_from("<I", body)
        found[rstr(body, 4)[0]] = name
    elif obj == SYNC:
        break

manager = next(
    (
        i
        for i in ("ext_data_control_manager_v1", "zwlr_data_control_manager_v1")
        if i in found
    ),
    None,
)
if not manager or "wl_seat" not in found:
    sys.exit("clip.py: compositor lacks data-control or a seat")

for iface, new_id in ((manager, MANAGER), ("wl_seat", SEAT)):
    request(
        REGISTRY,
        0,
        struct.pack("<I", found[iface]) + wstr(iface) + struct.pack("<II", 1, new_id),
    )
request(MANAGER, 0, struct.pack("<I", SOURCE))
for mime in offers:
    request(SOURCE, 0, wstr(mime))
request(MANAGER, 1, struct.pack("<II", DEVICE, SEAT))
request(DEVICE, 0, struct.pack("<I", SOURCE))

# Catch protocol errors or cancellation before reporting success.
request(1, 0, struct.pack("<I", SYNC2))
for obj, op, body in stream:
    if obj == SYNC2:
        break
    serve(obj, op, body)

if os.fork():
    os._exit(0)
os.setsid()
null = os.open(os.devnull, os.O_RDWR)
for fd in (0, 1, 2):
    os.dup2(null, fd)
for obj, op, body in stream:
    serve(obj, op, body)
