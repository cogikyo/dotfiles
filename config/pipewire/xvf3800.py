#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = ["pyusb"]
# ///
"""Keep the reSpeaker XVF3800's calibrated mic settings in device flash."""

import argparse
import errno
import struct
import sys
import time

import usb.core
import usb.util

VID, PID = 0x2886, 0x001A
TIMEOUT_MS = 1000
OK, RETRY = 0, 64
DROPPED = {errno.ENODEV, errno.EPIPE, errno.EIO, errno.ETIMEDOUT}

PARAMS = {
    "VERSION": (48, 0, "3B"),
    "REBOOT": (48, 7, "B"),
    "SAVE_CONFIGURATION": (48, 9, "B"),
    "PP_AGCONOFF": (17, 10, "i"),
    "PP_AGCGAIN": (17, 13, "f"),
}

DESIRED = {
    "PP_AGCONOFF": 0,
    # Disabling auto-gain freezes the current gain instead of resetting it.
    # Pinned to the gain used for the echo bench and voice-loop gate tuning.
    "PP_AGCGAIN": 3.9618678,
}


class Array:
    def __init__(self):
        self.dev = usb.core.find(idVendor=VID, idProduct=PID)
        if self.dev is None:
            sys.exit(f"xvf3800: no USB device {VID:04x}:{PID:04x}")

    def read(self, name):
        resid, cmdid, fmt = PARAMS[name]
        size = struct.calcsize("<" + fmt) + 1
        for _ in range(100):
            r = self.dev.ctrl_transfer(
                usb.util.CTRL_IN
                | usb.util.CTRL_TYPE_VENDOR
                | usb.util.CTRL_RECIPIENT_DEVICE,
                0,
                0x80 | cmdid,
                resid,
                size,
                TIMEOUT_MS,
            )
            if r[0] == OK:
                v = struct.unpack("<" + fmt, bytes(r[1:]))
                return v[0] if len(v) == 1 else v
            if r[0] != RETRY:
                raise RuntimeError(f"{name}: status {r[0]}")
            time.sleep(0.01)
        raise RuntimeError(f"{name}: device kept asking to retry")

    def write(self, name, value):
        resid, cmdid, fmt = PARAMS[name]
        self.dev.ctrl_transfer(
            usb.util.CTRL_OUT
            | usb.util.CTRL_TYPE_VENDOR
            | usb.util.CTRL_RECIPIENT_DEVICE,
            0,
            cmdid,
            resid,
            struct.pack("<" + fmt, value),
            TIMEOUT_MS,
        )

    def close(self):
        usb.util.dispose_resources(self.dev)


def matches(want, got):
    if isinstance(want, float):
        want = struct.unpack("<f", struct.pack("<f", want))[0]
    return want == got


def drift(arr):
    rows = [(name, want, arr.read(name)) for name, want in DESIRED.items()]
    print(f"firmware {'.'.join(map(str, arr.read('VERSION')))}")
    for name, want, got in rows:
        print(
            f"{name:<14} want {want:<10} have {got:<10} {'ok' if matches(want, got) else 'DRIFT'}"
        )
    return [(name, want) for name, want, got in rows if not matches(want, got)]


def check(arr):
    return 1 if drift(arr) else 0


def apply(arr):
    for name, want in drift(arr):
        arr.write(name, want)
        print(f"wrote {name} = {want}")
    if drift(arr):
        print("readback still drifts; not saving", file=sys.stderr)
        return 1
    arr.write("SAVE_CONFIGURATION", 1)
    print("saved to flash")
    return 0


def reboot(arr):
    try:
        arr.write("REBOOT", 1)
    except usb.core.USBError as e:
        if e.errno not in DROPPED:
            raise
    print("rebooting; the USB device drops for a few seconds")
    return 0


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("cmd", choices=["check", "apply", "reboot"])
    cmd = p.parse_args().cmd
    arr = Array()
    try:
        return {"check": check, "apply": apply, "reboot": reboot}[cmd](arr)
    except usb.core.USBError as e:
        sys.exit(f"xvf3800: {e}")
    finally:
        arr.close()


if __name__ == "__main__":
    sys.exit(main())
