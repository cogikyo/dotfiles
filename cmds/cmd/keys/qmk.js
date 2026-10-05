import { keys, aliases, wrappers, codes } from "./keycodes.js";

const CTRL = 0x01, SHIFT = 0x02, ALT = 0x04, GUI = 0x08, RIGHT = 0x10;

const modNames = { MOD_LCTL: 0x01, MOD_LSFT: 0x02, MOD_LALT: 0x04, MOD_LGUI: 0x08, MOD_RCTL: 0x11, MOD_RSFT: 0x12, MOD_RALT: 0x14, MOD_RGUI: 0x18, MOD_HYPR: 0x0f, MOD_MEH: 0x07 };

const layerOps = {
  MO: (n) => `Layer ${n} while held`,
  TG: (n) => `Toggle layer ${n}`,
  TT: (n) => `Layer ${n} while held; tap repeatedly to toggle`,
  OSL: (n) => `Layer ${n} for the next key`,
  TO: (n) => `Switch to layer ${n}`,
  DF: (n) => `Set default layer ${n}`,
  PDF: (n) => `Persistently set default layer ${n}`,
};

const short = {
  KC_ENTER: "⏎", KC_BSPACE: "⌫", KC_TAB: "⇥", KC_ESCAPE: "Esc", KC_SPACE: "␣", KC_DELETE: "Del", KC_INSERT: "Ins",
  KC_LEFT: "←", KC_RIGHT: "→", KC_UP: "↑", KC_DOWN: "↓", KC_PGUP: "PgUp", KC_PGDOWN: "PgDn", KC_HOME: "Home", KC_END: "End",
  KC_LSHIFT: "Shift", KC_RSHIFT: "Shift", KC_LCTRL: "Ctrl", KC_RCTRL: "Ctrl", KC_LALT: "Alt", KC_RALT: "AltGr", KC_LGUI: "Super", KC_RGUI: "Super",
  KC_CAPSLOCK: "Caps", KC_PSCREEN: "PrtSc", KC_APPLICATION: "Menu",
  KC_MPLY: "⏯", KC_MNXT: "⏭", KC_MPRV: "⏮", KC_MSTP: "⏹", KC_MUTE: "Mute", KC_VOLU: "Vol +", KC_VOLD: "Vol −", KC_BRIU: "Bri +", KC_BRID: "Bri −",
  KC_BTN1: "Click", KC_BTN2: "Right click", KC_BTN3: "Middle click", KC_BTN4: "Back", KC_BTN5: "Forward",
  KC_MS_U: "Mouse ↑", KC_MS_D: "Mouse ↓", KC_MS_L: "Mouse ←", KC_MS_R: "Mouse →",
  KC_WH_U: "Wheel ↑", KC_WH_D: "Wheel ↓", KC_WH_L: "Wheel ←", KC_WH_R: "Wheel →",
};

const names = {
  KC_LSHIFT: "Left Shift", KC_RSHIFT: "Right Shift", KC_LCTRL: "Left Ctrl", KC_RCTRL: "Right Ctrl",
  KC_LALT: "Left Alt", KC_RALT: "Right Alt (AltGr)", KC_LGUI: "Left Super", KC_RGUI: "Right Super",
};

const keysyms = {
  KC_ENTER: "return", KC_ESCAPE: "escape", KC_BSPACE: "backspace", KC_TAB: "tab", KC_SPACE: "space",
  KC_MINUS: "minus", KC_EQUAL: "equal", KC_LBRACKET: "bracketleft", KC_RBRACKET: "bracketright", KC_BSLASH: "backslash",
  KC_SCOLON: "semicolon", KC_QUOTE: "apostrophe", KC_GRAVE: "grave", KC_COMMA: "comma", KC_DOT: "period", KC_SLASH: "slash",
  KC_PSCREEN: "print", KC_INSERT: "insert", KC_HOME: "home", KC_PGUP: "prior", KC_DELETE: "delete", KC_END: "end", KC_PGDOWN: "next",
  KC_RIGHT: "right", KC_LEFT: "left", KC_DOWN: "down", KC_UP: "up",
  KC_MPLY: "xf86audioplay", KC_MNXT: "xf86audionext", KC_MPRV: "xf86audioprev", KC_MSTP: "xf86audiostop",
  KC_MUTE: "xf86audiomute", KC_VOLU: "xf86audioraisevolume", KC_VOLD: "xf86audiolowervolume",
  KC_BRIU: "xf86monbrightnessup", KC_BRID: "xf86monbrightnessdown", KC_CALC: "xf86calculator", KC_MYCM: "xf86explorer",
  KC_MAIL: "xf86mail", KC_WSCH: "xf86search", KC_WHOM: "xf86homepage", KC_WBAK: "xf86back", KC_WFWD: "xf86forward",
  KC_WREF: "xf86refresh", KC_WFAV: "xf86favorites", KC_EJCT: "xf86eject", KC_PWR: "xf86poweroff", KC_SLEP: "xf86sleep",
};

export function keysym(id) {
  if (keysyms[id]) return keysyms[id];
  const m = /^KC_([A-Z0-9]|F\d+)$/.exec(id ?? "");
  return m ? m[1].toLowerCase() : null;
}

export function modSet(mods) {
  return [[CTRL, "ctrl"], [SHIFT, "shift"], [ALT, "alt"], [GUI, "super"]].filter(([bit]) => mods & bit).map(([, name]) => name);
}

export function modLabel(mods, long = false) {
  const side = mods & RIGHT ? "R" : "";
  const parts = [];
  if (mods & CTRL) parts.push("Ctrl");
  if (mods & SHIFT) parts.push("Shift");
  if (mods & ALT) parts.push("Alt");
  if (mods & GUI) parts.push("Super");
  if (mods === 0x0f) return long ? `${side}Hyper` : "Hyper";
  if (mods === 0x07) return long ? `${side}Meh` : "Meh";
  return (long && side ? side + " " : "") + parts.join("+");
}

function canonical(id) {
  return keys[id] ? id : aliases[id];
}

function parseMods(expr) {
  let mods = 0;
  for (const part of String(expr).split("|").map((s) => s.trim())) {
    if (part in modNames) mods |= modNames[part];
    else if (/^(0x[0-9a-f]+|\d+)$/i.test(part)) mods |= Number(part);
    else return null;
  }
  return mods;
}

function split(args) {
  const out = [];
  let depth = 0, start = 0;
  for (let i = 0; i < args.length; i++) {
    if (args[i] === "(") depth++;
    else if (args[i] === ")") depth--;
    else if (args[i] === "," && depth === 0) {
      out.push(args.slice(start, i).trim());
      start = i + 1;
    }
  }
  out.push(args.slice(start).trim());
  return out;
}

function fromNumber(n) {
  if (codes[n]) return codes[n];
  const hi = n & 0xff00, lo = n & 0xff;
  const inner = codes[lo] ?? `0x${lo.toString(16)}`;
  if (n >= 0x0100 && n < 0x2000) return { mods: (n >> 8) & 0x1f, inner, tap: false };
  if (n >= 0x2000 && n < 0x4000) return { mods: (n >> 8) & 0x1f, inner, tap: true };
  if (n >= 0x4000 && n < 0x5000) return `LT(${(n >> 8) & 0xf}, ${inner})`;
  if (n >= 0x5000 && n < 0x5200) return `LM(${(n >> 5) & 0xf}, ${n & 0x1f})`;
  if (hi === 0x5200 || (n >= 0x5200 && n < 0x5300)) {
    const ops = [[0x5200, "TO"], [0x5220, "MO"], [0x5240, "DF"], [0x5260, "TG"], [0x5280, "OSL"], [0x52a0, "OSM"], [0x52c0, "TT"], [0x52e0, "PDF"]];
    const [base, op] = ops.findLast(([b]) => n >= b);
    return `${op}(${n - base})`;
  }
  if (hi === 0x5700) return `TD(${lo})`;
  if (hi === 0x7700) return `M${lo}`;
  if (n >= 0x7e00 && n < 0x7e40) return `USER${String(n - 0x7e00).padStart(2, "0")}`;
  return null;
}

function basicLegend(id) {
  const k = keys[id];
  if (k.printable) {
    const lines = k.label.split("\n");
    const main = lines.length > 1 ? lines[1] : lines[0];
    return { main, shift: lines.length > 1 ? lines[0] : "" };
  }
  return { main: short[id] ?? k.label.replace(/\n/g, " "), shift: "" };
}

function basicName(id) {
  const k = keys[id];
  if (names[id]) return names[id];
  if (k.printable) return basicLegend(id).main;
  return (k.tip ?? k.label.replace(/\n/g, " ")).replace(/ \((Windows|Laptop)\)$/, "");
}

function compact(d) {
  if (!d) return "";
  const main = d.legend.main.replace(/\n/g, " ");
  return d.legend.mods ? `${d.legend.mods} ${main}` : main;
}

function plain(id, mods = 0) {
  const k = keys[id];
  if (k.code > 0xff && k.code < 0x2000) {
    return plain(codes[k.code & 0xff], mods | ((k.code >> 8) & 0x1f));
  }
  const legend = basicLegend(id);
  const named = basicName(id);
  if (!mods) return { kind: "basic", base: id, mods: 0, legend, text: named };
  if ((mods & ~RIGHT) === SHIFT && legend.shift) {
    return { kind: "mods", base: id, mods, legend: { main: legend.shift, shift: "" }, text: `${legend.shift} (Shift+${named})` };
  }
  return { kind: "mods", base: id, mods, legend: { ...legend, shift: "", mods: modLabel(mods) }, text: `${modLabel(mods, true)}+${named}` };
}

const unknown = (raw) => ({ kind: "unknown", raw, base: null, mods: 0, legend: { main: String(raw), shift: "" }, text: `Unknown keycode ${raw}` });

export function decode(raw, ctx = {}) {
  const d = decodeInner(raw, ctx, 0);
  return { raw, ...d };
}

function decodeInner(raw, ctx, depth) {
  if (raw === -1 || raw === null || raw === undefined) return { kind: "none", base: null, mods: 0, legend: { main: "", shift: "" }, text: "Nothing" };
  if (typeof raw === "number") {
    const s = fromNumber(raw);
    if (s === null) return unknown(raw);
    if (typeof s === "object") return wrap(s.mods, s.inner, s.tap, raw, ctx, depth);
    return decodeInner(s, ctx, depth);
  }
  const str = String(raw).trim();
  if (/^0x[0-9a-f]+$/i.test(str)) return decodeInner(Number(str), ctx, depth);

  const id = canonical(str);
  if (id === "KC_NO") return { kind: "none", base: null, mods: 0, legend: { main: "", shift: "" }, text: "Nothing (KC_NO)" };
  if (id === "KC_TRNS") return { kind: "trns", base: null, mods: 0, legend: { main: "", shift: "" }, text: "Transparent" };
  if (id) {
    const k = keys[id];
    if (k.code !== undefined && k.code < 0x2000) return plain(id);
    return { kind: "special", base: null, mods: 0, legend: { main: k.label.replace(/\n/g, " "), shift: "" }, text: k.tip ?? k.label.replace(/\n/g, " ") };
  }

  let m = /^USER(\d+)$/.exec(str);
  if (m) {
    const n = Number(m[1]);
    const c = ctx.customKeycodes?.[n];
    if (!c) return { kind: "user", base: null, mods: 0, legend: { main: `User ${n}`, shift: "" }, text: `User keycode ${n}` };
    return { kind: "user", base: null, mods: 0, legend: { main: c.shortName ?? c.name, shift: "" }, text: c.title ?? c.name, name: c.name };
  }
  m = /^M(\d+)$/.exec(str);
  if (m) return { kind: "macro", base: null, mods: 0, legend: { main: `M${m[1]}`, shift: "" }, text: `Macro ${m[1]}` };

  m = /^([A-Z_0-9]+)\((.*)\)$/s.exec(str);
  if (!m) return unknown(raw);
  const [, fn, rest] = m;
  const args = split(rest);

  if (layerOps[fn] && args.length === 1 && /^\d+$/.test(args[0])) {
    const layer = Number(args[0]);
    return { kind: "layer", base: null, mods: 0, layer, op: fn, legend: { main: `${fn} ${layer}`, shift: "" }, text: layerOps[fn](layer) };
  }
  if (fn === "TD" && /^\d+$/.test(args[0])) return tapDance(Number(args[0]), ctx, depth);
  if (fn === "OSM") {
    const mods = parseMods(args[0]);
    if (mods === null) return unknown(raw);
    return { kind: "special", base: null, mods: 0, legend: { main: `OSM ${modLabel(mods)}`, shift: "" }, text: `One-shot ${modLabel(mods, true)}` };
  }
  if (fn === "LM" && args.length === 2) {
    const mods = parseMods(args[1]);
    if (mods === null) return unknown(raw);
    return { kind: "layer", base: null, mods: 0, layer: Number(args[0]), op: "LM", legend: { main: `LM ${args[0]}`, shift: "", mods: modLabel(mods) }, text: `Hold for layer ${args[0]} with ${modLabel(mods, true)}` };
  }
  if (fn === "MT" && args.length === 2) {
    const mods = parseMods(args[0]);
    if (mods === null) return unknown(raw);
    return wrap(mods, args[1], true, raw, ctx, depth);
  }
  const lt = /^LT(\d+)$/.exec(fn);
  if ((lt && args.length === 1) || (fn === "LT" && args.length === 2)) {
    const layer = Number(lt ? lt[1] : args[0]);
    const tap = decodeInner(lt ? args[0] : args[1], ctx, depth + 1);
    return { ...tap, kind: "layertap", layer, hold: `L${layer}`, legend: { ...tap.legend, hold: `L${layer}` }, text: `${tap.text} on tap, layer ${layer} while held` };
  }
  if (wrappers[fn] && args.length === 1) return wrap(wrappers[fn].mods, args[0], wrappers[fn].tap, raw, ctx, depth);
  return unknown(raw);
}

function wrap(mods, inner, tap, raw, ctx, depth) {
  const d = decodeInner(inner, ctx, depth + 1);
  if (d.kind === "unknown") return unknown(raw);
  if (tap) {
    const hold = modLabel(mods);
    return { ...d, kind: "modtap", hold, holdMods: mods, legend: { ...d.legend, hold }, text: `${d.text} on tap, ${modLabel(mods, true)} while held` };
  }
  if (d.base) return plain(d.base, d.mods | mods);
  return unknown(raw);
}

function tapDance(n, ctx, depth) {
  const td = ctx.tapDance?.[n];
  const fallback = { kind: "td", td: n, base: null, mods: 0, legend: { main: `TD ${n}`, shift: "" }, text: `Tap dance ${n}` };
  if (!td || depth > 2) return fallback;
  const [tap, hold, double, tapHold, term] = td;
  const actions = { tap, hold, double, tapHold };
  const decoded = {};
  for (const [k, v] of Object.entries(actions)) {
    const d = decodeInner(v, ctx, depth + 1);
    if (d.kind !== "none") decoded[k] = d;
  }
  const lead = decoded.tap ?? decoded.hold ?? decoded.double ?? decoded.tapHold;
  if (!lead) return fallback;
  const parts = Object.entries(decoded).map(([k, d]) => `${{ tap: "tap", hold: "hold", double: "double-tap", tapHold: "tap-hold" }[k]}: ${d.text}`);
  return {
    kind: "td",
    td: n,
    term,
    actions: decoded,
    base: decoded.tap?.base ?? null,
    mods: decoded.tap?.mods ?? 0,
    legend: { ...(decoded.tap ?? lead).legend, td: `TD${n}`, hold: compact(decoded.hold), double: compact(decoded.double) },
    text: `Tap dance ${n} — ${parts.join("; ")}`,
  };
}
