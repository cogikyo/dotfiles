import builtins
import json
import os
import shlex
import sys

from kitty.config import load_config
from kitty.constants import defconf
from kitty.key_encoding import functional_key_number_to_name_map

opened = set()
real_open = builtins.open


def tracking_open(file, *args, **kwargs):
    if isinstance(file, (str, bytes, os.PathLike)):
        opened.add(os.path.abspath(os.fsdecode(file)))
    return real_open(file, *args, **kwargs)


builtins.open = tracking_open
try:
    opts = load_config(defconf)
finally:
    builtins.open = real_open

GLFW_MODS = (("ctrl", 4), ("shift", 1), ("alt", 2), ("super", 8))


def chord(k):
    if k.is_native:
        name = f"native:{k.key}"
    elif k.key in functional_key_number_to_name_map:
        name = functional_key_number_to_name_map[k.key]
    else:
        name = chr(k.key)
    return {"mods": [m for m, bit in GLFW_MODS if k.mods & bit], "key": name}


def words(text):
    return " ".join(w.replace("_", " ") for w in text)


def label(action):
    try:
        parts = shlex.split(action)
    except ValueError:
        parts = action.split()
    if not parts:
        return ""
    name, args = parts[0], [a for a in parts[1:] if not a.startswith("--")]
    flags = dict(
        a[2:].split("=", 1) for a in parts[1:] if a.startswith("--") and "=" in a
    )
    if name == "goto_tab" and args:
        return f"Tab {args[0]}"
    if name == "scroll_to_prompt" and args:
        return "Previous prompt" if args[0].startswith("-") else "Next prompt"
    if name == "change_font_size" and args:
        size = args[-1]
        if size in ("0", "0.0"):
            return "Reset font size"
        return f"Font size {size.removesuffix('.0')}"
    if name == "kitten" and args:
        return words([os.path.splitext(args[0])[0]]).capitalize()
    if "--type" in parts[:-1]:
        flags["type"] = parts[parts.index("--type") + 1]
    if "type" in flags:
        return f"{words([name]).capitalize()}: {flags['type']}"
    return words([name]).capitalize()


def keys(d):
    return (d.trigger, *d.rest)


def resolve(defs, depth, shadow):
    groups = {}
    for d in defs:
        groups.setdefault(keys(d)[depth], []).append(d)
    for group in groups.values():
        last = max((i for i, d in enumerate(group) if len(keys(d)) == depth + 1), default=-1)
        live = group[last:] if last == len(group) - 1 else group[last + 1 :]
        for d in group:
            if not any(d is w for w in live):
                shadow[id(d)] = max(live, key=lambda w: common(keys(w), keys(d)))
        if len(live) > 1 or len(keys(live[0])) > depth + 1:
            resolve(live, depth + 1, shadow)


def common(a, b):
    n = 0
    while n < min(len(a), len(b)) and a[n] == b[n]:
        n += 1
    return n


defs = [
    d
    for items in opts.keyboard_modes[""].keymap.values()
    for d in items
    if not d.options.when_focus_on
]
shadow = {}
resolve(defs, 0, shadow)

binds = []
for d in defs:
    k = keys(d)
    bind = {
        "prefix": [chord(x) for x in k[:-1]],
        **chord(k[-1]),
        "label": label(d.definition),
        "detail": d.definition,
        "builtin": not d.definition_location.file,
    }
    winner = shadow.get(id(d))
    while winner is not None and id(winner) in shadow:
        winner = shadow[id(winner)]
    if winner is not None:
        bind["shadowedBy"] = [chord(x) for x in keys(winner)]
    binds.append(bind)

json.dump(
    {
        "binds": binds,
        "files": sorted(opened),
    },
    sys.stdout,
)
