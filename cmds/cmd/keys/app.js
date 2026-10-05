import { names } from "./names.js";
import { decode, keysym, modSet } from "./qmk.js";

const POLL_MS = 1000;
const MODS = ["ctrl", "shift", "alt", "super"];
const GAP = 0.5;
const SPLIT = 1.5;
const LABEL = 0.32;
const TUCK = 0.2;
const FINGERS = [
  { name: "index", drop: 0.65 },
  { name: "middle", drop: 0 },
  { name: "ring", drop: 0.35 },
  { name: "pinky", drop: 1.3 },
];
const LAYERS = { 0: "Base", 1: "Symbols", 2: "System", 14: "Config", 15: "Mouse" };
const OPS = { TG: "toggle", TT: "tap-toggle", OSL: "one-shot", TO: "go to", DF: "default", PDF: "default" };
const DANCE = { tap: "tap", hold: "hold", double: "double-tap", tapHold: "tap-hold" };
const PLUS = [
  { name: "South", at: [0, 1] },
  { name: "East", at: [1, 0] },
  { name: "Center", at: [0, 0] },
  { name: "North", at: [0, -1] },
  { name: "West", at: [-1, 0] },
];
const THUMB = [
  { name: "Knuckle", at: [1.02, 0.7] },
  { name: "Nail", at: [1.02, -0.3] },
  { name: "Down", at: [0, 0], h: 1.6 },
  { name: "Pad", at: [-1.12, -0.3], w: 1.25 },
  { name: "Up", at: [-1.25, 0.7], w: 1.5 },
];
const ICONS = { keys: "\u{F030C}", hyprland: "\u{F359}", kitty: "\u{F011B}", nvim: "\u{E6AE}", opencode: "\u{F0BC9}" };
const MOD_KEYS = new Set(["Control", "Shift", "Alt", "AltGraph", "Meta", "OS", "Super", "Hyper"]);
const modNames = { ctrl: "Ctrl", shift: "Shift", alt: "Alt", super: "Super" };
const keyNames = {
  space: "Space", return: "Enter", escape: "Esc", backspace: "Bksp", tab: "Tab", delete: "Del", insert: "Ins",
  home: "Home", end: "End", prior: "PgUp", next: "PgDn", left: "←", right: "→", up: "↑", down: "↓", print: "Print",
  minus: "-", equal: "=", bracketleft: "[", bracketright: "]", backslash: "\\", semicolon: ";", apostrophe: "'",
  grave: "`", comma: ",", period: ".", slash: "/",
};
const codeIds = {
  Minus: "KC_MINUS", Equal: "KC_EQUAL", BracketLeft: "KC_LBRACKET", BracketRight: "KC_RBRACKET", Backslash: "KC_BSLASH",
  Semicolon: "KC_SCOLON", Quote: "KC_QUOTE", Backquote: "KC_GRAVE", Comma: "KC_COMMA", Period: "KC_DOT", Slash: "KC_SLASH",
  Space: "KC_SPACE", Enter: "KC_ENTER", Escape: "KC_ESCAPE", Backspace: "KC_BSPACE", Tab: "KC_TAB", Delete: "KC_DELETE",
  Insert: "KC_INSERT", Home: "KC_HOME", End: "KC_END", PageUp: "KC_PGUP", PageDown: "KC_PGDOWN", ArrowLeft: "KC_LEFT",
  ArrowRight: "KC_RIGHT", ArrowUp: "KC_UP", ArrowDown: "KC_DOWN", PrintScreen: "KC_PSCREEN", CapsLock: "KC_CAPSLOCK",
  ShiftLeft: "KC_LSHIFT", ShiftRight: "KC_RSHIFT", ControlLeft: "KC_LCTRL", ControlRight: "KC_RCTRL", AltLeft: "KC_LALT",
  AltRight: "KC_RALT", MetaLeft: "KC_LGUI", MetaRight: "KC_RGUI", MediaPlayPause: "KC_MPLY", MediaTrackNext: "KC_MNXT",
  MediaTrackPrevious: "KC_MPRV", AudioVolumeMute: "KC_MUTE", AudioVolumeUp: "KC_VOLU", AudioVolumeDown: "KC_VOLD",
  LaunchApp1: "KC_MYCM", LaunchApp2: "KC_CALC",
};

const ui = { app: "keys", layer: 0, views: {}, peek: null, pressed: new Set(), hover: null, scoped: false };
let data = null;
let model = null;
let indexes = new Map();
let version = "";
let offline = "";

const $ = (id) => document.getElementById(id);

function el(tag, cls, text) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

const norm = (mods) => MODS.filter((m) => (mods ?? []).includes(m));
const chordKey = (c) => `${norm(c.mods).join("+")}:${c.key}`;
const pathKey = (path) => path.map(chordKey).join(" ");
const keyName = (k) => keyNames[k] ?? (k.startsWith("xf86") ? k.slice(4) : k.toUpperCase());
const comboText = (c) => [...norm(c.mods).map((m) => modNames[m]), keyName(c.key)].join("+");
const seqText = (seq) => seq.map(comboText).join(" › ");
const chipText = (id) => (id ? id.split("+").map((m) => modNames[m]).join("+") : "none");
const layerName = (n) => LAYERS[n] ?? String(n);
const entry = (label) => (label && Object.hasOwn(names[ui.app] ?? {}, label) ? names[ui.app][label] : undefined);
const named = (label) => entry(label)?.name ?? label;
const groupName = (g) => named(g.label) ?? comboText(g.chord);

function codeId(code) {
  const m = /^(?:Key|Digit)([A-Z0-9])$/.exec(code) ?? /^(F\d+)$/.exec(code);
  return m ? `KC_${m[1]}` : codeIds[code];
}

function parseKLE(rows) {
  const keys = [];
  let y = 0;
  for (const row of rows) {
    if (!Array.isArray(row)) continue;
    let x = 0, w = 1, h = 1, decal = false;
    for (const item of row) {
      if (typeof item === "object") {
        x += item.x ?? 0;
        y += item.y ?? 0;
        w = item.w ?? w;
        h = item.h ?? h;
        decal = item.d ?? decal;
        continue;
      }
      const lines = String(item).split("\n");
      const matrix = /^(\d+),(\d+)$/.exec(lines[0] ?? "");
      const option = /^(\d+),(\d+)$/.exec(lines[3] ?? "");
      if (!decal && matrix) {
        keys.push({
          x, y, w, h,
          row: Number(matrix[1]),
          col: Number(matrix[2]),
          option: option ? [Number(option[1]), Number(option[2])] : null,
        });
      }
      x += w;
      w = 1; h = 1; decal = false;
    }
    y += 1;
  }
  return keys;
}

function layoutChoices(labels = [], packed = 0) {
  const widths = labels.map((l) => {
    if (typeof l === "string") return 1;
    const options = l.length - 1;
    return options <= 1 ? 0 : (options - 1).toString(2).length;
  });
  const choices = [];
  let value = BigInt(packed);
  for (let i = labels.length - 1; i >= 0; i--) {
    const bits = BigInt(widths[i]);
    choices[i] = Number(value & ((1n << bits) - 1n));
    value >>= bits;
  }
  return choices;
}

function isEmpty(raw) {
  return raw === "KC_TRNS" || raw === "KC_NO" || raw === -1 || raw === 0 || raw === 1;
}

function bbox(items) {
  const x0 = Math.min(...items.map((k) => k.x)), y0 = Math.min(...items.map((k) => k.y));
  return { x0, y0, w: Math.max(...items.map((k) => k.x + (k.w ?? 0))) - x0, h: Math.max(...items.map((k) => k.y + (k.h ?? 0))) - y0 };
}

function clusters() {
  const fingers = FINGERS.map(({ name, drop }, i) => {
    const x = (FINGERS.length - 1 - i) * (3 + GAP);
    return { name, x, y: drop, w: 3, h: 3, origin: [x + 1.5, drop + 1.5] };
  });
  const t = bbox(THUMB.map(({ at: [x, y], w = 1, h = 1 }) => ({ x: x - w / 2, y: y - h / 2, w, h })));
  const index = fingers[0];
  const x = index.x + index.w - t.w, y = index.y + index.h + LABEL + TUCK;
  const left = [{ name: "thumb", x, y, w: t.w, h: t.h, origin: [x - t.x0, y - t.y0] }, ...fingers];
  const mirror = 2 * (index.x + index.w) + SPLIT;
  return [
    ...left.map((c) => ({ ...c, side: 0, flip: 1 })),
    ...left.map((c, i) => ({ ...c, side: 1, flip: i ? 1 : -1, x: mirror - c.x - c.w, origin: [mirror - c.origin[0], c.origin[1]] })),
  ].map((c) => ({ ...c, label: `${c.side ? "R" : "L"} ${c.name}` }));
}

function place(k, plates) {
  const plate = plates[k.row];
  const spot = (k.row % 5 ? PLUS : THUMB)[k.col];
  if (!plate || !spot) {
    if (k.col === 5) return false;
    throw new Error(`no position for matrix ${k.id}`);
  }
  const w = spot.w ?? 1, h = spot.h ?? 1;
  const cx = plate.origin[0] + plate.flip * spot.at[0], cy = plate.origin[1] + spot.at[1];
  return Object.assign(k, { side: plate.side, x: cx - w / 2, y: cy - h / 2, w, h, round: k.row % 5 > 0 && k.col === 2 });
}

function build(d) {
  const def = d.definition;
  const vil = d.vil;
  const all = parseKLE(def.layouts?.keymap ?? []);
  const choices = layoutChoices(def.layouts?.labels, vil.layout_options ?? 0);
  const plates = clusters();
  const keys = all.filter((k) => !k.option || choices[k.option[0]] === k.option[1]).filter((k) => {
    k.id = `${k.row},${k.col}`;
    return place(k, plates);
  });
  const box = bbox(plates.map((p) => ({ ...p, h: p.h + LABEL })));
  const ctx = { customKeycodes: def.customKeycodes, tapDance: vil.tap_dance };
  const layers = vil.layout.map((rows, i) => i === 0 || keys.some((k) => !isEmpty(rows[k.row]?.[k.col])) ? i : -1).filter((i) => i >= 0);
  const decoded = vil.layout.map((rows) => keys.map((k) => decode(rows[k.row]?.[k.col], ctx)));
  const cell = (layer, k) => {
    const i = keys.indexOf(k);
    const own = decoded[layer][i];
    const base = decoded[0][i];
    if (own.kind !== "trns" || base.kind === "trns") return { own, eff: own, from: layer };
    return { own, eff: base, from: 0 };
  };
  const syms = new Set(layers.flatMap((l) => keys.map((k) => keysym(cell(l, k).eff.base))).filter(Boolean));
  return { keys, plates, box, layers, cell, names: syms };
}

const rank = (b) => (b.shadowedBy ? 8 : 0) + (b.scope ? 4 : 0) + (b.builtin ? 2 : 0);

function indexApp(app) {
  const leaves = new Map();
  const groups = new Map();
  const labels = new Map((app.groups ?? []).map((g) => [pathKey(g.prefix ?? []), g.label]));
  const level = (m, k) => m.get(k) ?? m.set(k, new Map()).get(k);
  for (const b of app.binds ?? []) {
    if (b.scope && !ui.scoped) continue;
    const prefix = b.prefix ?? [];
    const bind = { ...b, prefix, mods: norm(b.mods) };
    const at = level(leaves, pathKey(prefix));
    const ck = chordKey(bind);
    at.set(ck, [...(at.get(ck) ?? []), bind].sort((x, y) => rank(x) - rank(y)));
    prefix.forEach((c, i) => {
      const at = level(groups, pathKey(prefix.slice(0, i)));
      const k = chordKey(c);
      const g = at.get(k) ?? { chord: { mods: norm(c.mods), key: c.key }, count: 0, user: 0, label: labels.get(pathKey(prefix.slice(0, i + 1))) };
      g.count++;
      g.user += !b.builtin;
      at.set(k, g);
    });
  }
  return { app, leaves, groups, scoped: (app.binds ?? []).filter((b) => b.scope).length };
}

function reindex() {
  indexes = new Map((data?.apps ?? []).map((a) => [a.id, indexApp(a)]));
}

const appList = () => [{ id: "keys", name: "Keys" }, ...(data?.apps ?? [])];
const index = () => indexes.get(ui.app);

function view() {
  ui.views[ui.app] ??= { path: [], stack: [], chip: null };
  return ui.views[ui.app];
}

function chips(path) {
  const ix = index();
  if (!ix || !model) return [];
  const pk = pathKey(path);
  const counts = new Map();
  const add = (mods, key) => {
    if (!model.names.has(key)) return;
    const id = norm(mods).join("+");
    counts.set(id, (counts.get(id) ?? 0) + 1);
  };
  for (const binds of ix.leaves.get(pk)?.values() ?? []) for (const b of binds) add(b.mods, b.key);
  for (const g of ix.groups.get(pk)?.values() ?? []) add(g.chord.mods, g.chord.key);
  const order = (id) => (id ? id.split("+").reduce((n, m) => n | (1 << MODS.indexOf(m)), 0) : 0);
  return [...counts].map(([id, count]) => ({ id, count })).sort((a, b) => order(a.id) - order(b.id));
}

function settle() {
  if (!appList().some((a) => a.id === ui.app)) ui.app = "keys";
  if (!model.layers.includes(ui.layer)) ui.layer = model.layers[0];
  if (ui.app === "keys") return;
  const v = view();
  while (v.path.length && !index().groups.get(pathKey(v.path.slice(0, -1)))?.has(chordKey(v.path.at(-1)))) up();
  const list = chips(v.path);
  if (!list.some((c) => c.id === v.chip)) v.chip = list.reduce((best, c) => (c.count > (best?.count ?? -1) ? c : best), null)?.id ?? "";
}

function unpeek() {
  if (ui.peek === null || ui.app === "keys") return;
  view().chip = ui.peek;
  ui.peek = null;
}

function drill(chord) {
  unpeek();
  const v = view();
  v.stack.push(v.chip);
  v.path = [...v.path, chord];
  const list = chips(v.path);
  if (list.some((c) => c.id === "")) v.chip = "";
  else if (!list.some((c) => c.id === v.chip)) v.chip = list[0]?.id ?? "";
}

function up() {
  const v = view();
  if (!v.path.length) return false;
  unpeek();
  v.path = v.path.slice(0, -1);
  v.chip = v.stack.pop() ?? v.chip;
  return true;
}

function hit(eff) {
  const ix = index();
  const key = keysym(eff.base);
  if (!ix || !key) return null;
  const v = view();
  const pk = pathKey(v.path);
  const chord = { mods: norm([...(v.chip ? v.chip.split("+") : []), ...modSet(eff.mods)]), key };
  const ck = chordKey(chord);
  return { chord, leaves: ix.leaves.get(pk)?.get(ck) ?? [], group: ix.groups.get(pk)?.get(ck) };
}

function winner(seq) {
  const ix = index();
  const pk = pathKey(seq.slice(0, -1));
  const ck = chordKey(seq.at(-1));
  const leaf = ix?.leaves.get(pk)?.get(ck)?.find((b) => !b.shadowedBy);
  const group = ix?.groups.get(pk)?.get(ck);
  return leaf ? named(leaf.label) || leaf.detail : group ? groupName(group) : "";
}

function render() {
  if (!model) return;
  settle();
  document.body.dataset.app = ui.app;
  renderApps();
  renderLayers();
  renderChips();
  renderCrumbs();
  renderStatus();
  renderBoard();
}

function switchApp(id) {
  unpeek();
  ui.app = id;
}

function renderApps() {
  $("apps").replaceChildren(...appList().map((a) => {
    const b = el("button", a.id === ui.app ? "on" : "");
    b.append(el("span", "icon", ICONS[a.id] ?? ""), a.name);
    b.dataset.app = a.id;
    if (a.error) b.classList.add("broken");
    b.onclick = () => { switchApp(a.id); render(); };
    return b;
  }));
}

function renderLayers() {
  $("layers").replaceChildren(...model.layers.map((layer, i) => {
    const b = el("button", layer === ui.layer ? "on" : "", layerName(layer));
    b.dataset.layer = layer;
    if (i < 9) b.dataset.kbd = String(i + 1);
    b.onclick = () => { ui.layer = layer; render(); };
    return b;
  }));
}

function renderChips() {
  const nav = $("chips");
  const toggle = $("scoped");
  const scoped = index()?.scoped ?? 0;
  toggle.hidden = ui.app === "keys" || !scoped;
  toggle.classList.toggle("on", ui.scoped);
  toggle.replaceChildren("scoped", el("span", "count", String(scoped)));
  if (ui.app === "keys") return nav.replaceChildren();
  const v = view();
  nav.replaceChildren(...chips(v.path).map((c) => {
    const b = el("button", c.id === v.chip ? "on" : "", chipText(c.id));
    if (c.id === v.chip && ui.peek !== null) b.classList.add("peek");
    b.append(el("span", "count", String(c.count)));
    b.onclick = () => { v.chip = c.id; ui.peek = null; render(); };
    return b;
  }));
}

function renderCrumbs() {
  const nav = $("crumbs");
  const v = ui.app === "keys" ? null : view();
  if (!v?.path.length) return nav.replaceChildren();
  const root = el("button", "", "root");
  root.onclick = () => { while (up()); render(); };
  nav.replaceChildren(root, ...v.path.flatMap((c, i) => {
    const b = el("button", i === v.path.length - 1 ? "on" : "", comboText(c));
    b.onclick = () => { while (v.path.length > i + 1) up(); render(); };
    return [el("span", "sep", "›"), b];
  }));
}

function renderStatus() {
  const box = $("status");
  const app = data?.apps?.find((a) => a.id === ui.app);
  const lines = [
    offline && ["error", `server unreachable, showing last state (${offline})`],
    data?.definitionError && ["error", `definition: ${data.definitionError}`],
    data?.vilError && ["error", `keymap: ${data.vilError}`],
    app?.error && ["error", app.error],
    app?.note && ["note", app.note],
  ].filter(Boolean);
  box.replaceChildren(...lines.map(([cls, text]) => el("span", cls, text)));
}

const measure = document.createElement("canvas").getContext("2d");

function wrap(text, size, width, max) {
  measure.font = `500 ${size}px Vagari`;
  const fits = (s) => measure.measureText(s).width <= width;
  const words = text.split(/\s+/).filter(Boolean).flatMap((w) => w.split(/(?<=\/)(?=.)/));
  const join = (a, b) => (a.endsWith("/") ? a + b : `${a} ${b}`);
  const lines = [];
  while (words.length && lines.length < max) {
    let line = words.shift();
    while (words.length && fits(join(line, words[0]))) line = join(line, words.shift());
    lines.push(line);
  }
  if (words.length) {
    const tail = lines.pop().split(" ");
    while (tail.length > 1 && !fits(`${tail.join(" ")}…`)) tail.pop();
    lines.push(`${tail.join(" ")}…`);
  }
  return {
    clipped: lines.some((l) => !fits(l)),
    lines: lines.map((l) => {
      if (fits(l)) return l;
      let s = l.replace(/…$/, "");
      while (s && !fits(`${s}…`)) s = s.slice(0, -1);
      return `${s.trimEnd()}…`;
    }),
  };
}

function legend(text, u, w, h) {
  const base = Math.max(11, Math.min(24, u * 0.17));
  const width = w - u * 0.14;
  let best = null;
  for (const size of [base, base * 0.86, base * 0.74]) {
    const max = Math.max(1, Math.min(3, Math.floor((h - u * 0.34) / (size * 1.15))));
    best = { size, ...wrap(text, size, width, max) };
    if (!best.clipped && !best.lines.at(-1).endsWith("…")) break;
  }
  const box = el("div", "legend");
  box.style.fontSize = `${best.size}px`;
  box.append(...best.lines.map((l) => el("span", "", l)));
  return box;
}

function renderBoard() {
  const board = $("board");
  const stage = $("stage");
  const { u, width, height, x, y } = fit(stage.clientWidth, innerHeight / 3);
  board.style.setProperty("--u", `${u}px`);
  board.dataset.layer = ui.layer;
  board.style.width = `${width * u}px`;
  board.style.height = `${height * u}px`;
  board.replaceChildren(...model.plates.map((p) => {
    const label = el("div", "label", p.label);
    Object.assign(label.style, { left: `${x(p.x)}px`, top: `${y(p.y + p.h)}px`, width: `${p.w * u}px`, height: `${LABEL * u}px` });
    return label;
  }));

  const app = ui.app !== "keys";
  for (const k of model.keys) {
    const { eff, from } = model.cell(ui.layer, k);
    const key = el("div", `key kind-${eff.kind}`);
    key.dataset.id = k.id;
    if (eff.base) key.dataset.base = eff.base;
    const w = k.w * u, h = k.h * u;
    const room = k.round ? [w - u * 0.08, h - u * 0.2] : [w, h];
    Object.assign(key.style, { left: `${x(k.x)}px`, top: `${y(k.y)}px`, width: `${w}px`, height: `${h}px` });
    if (k.round) key.classList.add("round");
    key.append(cap(eff));
    if (from !== ui.layer) key.classList.add("inherited");
    const hitting = app ? hit(eff) : null;
    if (app) key.classList.add(hitting?.leaves.length || hitting?.group ? "bound" : "free");
    const { leaves = [], group } = hitting ?? {};
    const first = leaves[0];
    if (first) {
      key.append(legend(named(first.label) || first.detail || comboText(hitting.chord), u, ...room));
      key.classList.toggle("builtin", leaves.every((b) => b.builtin));
      key.classList.toggle("scoped", !!first.scope);
      key.classList.toggle("dead", !!first.shadowedBy);
      key.classList.toggle("multi", leaves.length > 1);
    } else if (group) {
      if (group.label) key.append(legend(named(group.label), u, ...room));
      else key.classList.add("bare");
      key.classList.add("group");
      key.classList.toggle("builtin", !group.user);
    }
    if (group) {
      key.append(el("span", "opens", "›"));
      key.classList.add("drills");
      key.onclick = () => { drill(group.chord); render(); };
    }
    key.onmouseenter = () => { ui.hover = k.id; tip(); };
    key.onmouseleave = () => { ui.hover = null; tip(); };
    board.append(key);
  }
  markPressed();
  tip();
}

function fit(stageW, stageH) {
  const pad = 0.12;
  const { x0, y0, w, h } = model.box;
  const width = w + pad * 2, height = h + pad * 2;
  const u = Math.min(stageW / width, stageH / height);
  return { u, width, height, x: (v) => (v - x0 + pad) * u, y: (v) => (v - y0 + pad) * u };
}

function cap(eff) {
  const c = el("div", "cap");
  let lg = eff.legend;
  if (eff.kind === "layer") lg = { ...lg, main: layerName(eff.layer), mods: [lg.mods, OPS[eff.op]].filter(Boolean).join(" ") };
  if (eff.kind === "layertap") lg = { ...lg, hold: layerName(eff.layer) };
  if (lg.shift) c.append(el("span", "shift", lg.shift));
  if (lg.mods) c.append(el("span", "mods", lg.mods));
  const main = el("span", "main", lg.main);
  if (lg.main.length > 3 || lg.main.includes("\n")) main.classList.add("long");
  c.append(main);
  if (lg.hold) c.append(el("span", "hold", lg.hold));
  if (lg.double) c.append(el("span", "double", `2× ${lg.double}`));
  return c;
}

function notes(eff) {
  if (ui.app === "keys") {
    if (eff.kind === "user" && eff.name && eff.text !== eff.name) return [el("div", "about", eff.text)];
    if (eff.kind !== "td" || !eff.actions) return [];
    return Object.entries(eff.actions).map(([k, d]) => {
      const row = el("div", "row");
      row.append(el("span", "tag", DANCE[k]), d.text);
      return row;
    });
  }
  const h = hit(eff);
  if (!h) return [];
  const rows = h.leaves.flatMap((b) => {
    const about = entry(b.label)?.about;
    if (!about && !b.scope && !b.shadowedBy) return [];
    const row = el("div", "row");
    if (h.leaves.length > 1) row.append(el("div", "name", named(b.label) || b.detail || comboText(h.chord)));
    if (about) row.append(el("div", "about", about));
    if (b.scope) row.append(el("div", "scope", `only in ${b.scope}`));
    if (b.shadowedBy) {
      const by = winner(b.shadowedBy);
      row.append(el("div", "shadow", `shadowed by ${seqText(b.shadowedBy)}${by ? ` · ${by}` : ""}`));
    }
    return [row];
  });
  const about = h.group && entry(h.group.label)?.about;
  if (about) rows.push(el("div", "about", about));
  return rows;
}

function tip() {
  const box = $("tip");
  const k = model.keys.find((x) => x.id === ui.hover);
  const node = k && $("board").querySelector(`[data-id="${k.id}"]`);
  const rows = node ? notes(model.cell(ui.layer, k).eff) : [];
  if (!rows.length) return box.classList.remove("on");
  box.replaceChildren(...rows);
  box.classList.add("on");
  const r = node.getBoundingClientRect();
  const t = box.getBoundingClientRect();
  const left = Math.max(8, Math.min(innerWidth - t.width - 8, r.left + r.width / 2 - t.width / 2));
  const top = r.bottom + 8 + t.height < innerHeight ? r.bottom + 8 : Math.max(8, r.top - t.height - 8);
  Object.assign(box.style, { left: `${left}px`, top: `${top}px` });
}

function markPressed() {
  const ids = new Set([...ui.pressed].map(codeId));
  for (const key of $("board").querySelectorAll(".key")) key.classList.toggle("pressed", ids.has(key.dataset.base));
}

function heldMods(ev) {
  return [ev.ctrlKey && "ctrl", ev.shiftKey && "shift", ev.altKey && "alt", ev.metaKey && "super"].filter(Boolean).join("+");
}

function peek(held) {
  if (ui.app === "keys") return;
  const v = view();
  if (held === "") {
    if (ui.peek === null) return;
    unpeek();
    return render();
  }
  if (!chips(v.path).some((c) => c.id === held)) return;
  ui.peek ??= v.chip;
  v.chip = held;
  render();
}

function cycle(list, current, step) {
  const i = list.indexOf(current);
  return list[(i + step + list.length) % list.length];
}

function press(ev) {
  if (ev.key === "Tab") {
    if (ev.ctrlKey || ev.altKey || ev.metaKey) return false;
    switchApp(cycle(appList().map((a) => a.id), ui.app, ev.shiftKey ? -1 : 1));
    return true;
  }
  if (ev.ctrlKey || ev.altKey || ev.metaKey || ev.shiftKey) return false;
  if (/^[1-9]$/.test(ev.key)) {
    const layer = model.layers[Number(ev.key) - 1];
    if (layer === undefined) return false;
    ui.layer = layer;
  } else if (ev.key === "[" || ev.key === "]") {
    if (ui.app === "keys") return false;
    unpeek();
    const v = view();
    v.chip = cycle(chips(v.path).map((c) => c.id), v.chip, ev.key === "]" ? 1 : -1) ?? v.chip;
  } else if (ev.key === "Escape" || ev.key === "Backspace") {
    if (ui.app === "keys" || !up()) return false;
  } else {
    const key = keysym(codeId(ev.code));
    const group = ui.app !== "keys" && key && index().groups.get(pathKey(view().path))?.get(chordKey({ mods: [], key }));
    if (group) drill(group.chord);
    else return false;
  }
  return true;
}

$("scoped").onclick = () => {
  ui.scoped = !ui.scoped;
  reindex();
  render();
};
document.addEventListener("mousedown", (ev) => {
  if (ev.target.closest("button")) ev.preventDefault();
});
document.addEventListener("keydown", (ev) => {
  if (ev.key === "Tab") ev.preventDefault();
  ui.pressed.add(ev.code);
  if (model) markPressed();
  if (!model || ev.repeat) return;
  if (MOD_KEYS.has(ev.key)) return peek(heldMods(ev));
  if (!press(ev)) return;
  ev.preventDefault();
  render();
});
document.addEventListener("keyup", (ev) => {
  ui.pressed.delete(ev.code);
  if (!model) return;
  markPressed();
  if (MOD_KEYS.has(ev.key)) peek(heldMods(ev));
});
window.addEventListener("blur", () => {
  ui.pressed.clear();
  if (!model) return;
  markPressed();
  peek("");
});
window.addEventListener("resize", () => { if (model) renderBoard(); });
document.fonts.addEventListener("loadingdone", () => { if (model) renderBoard(); });

async function poll() {
  try {
    const res = await fetch(`/api/state?v=${version}`, { cache: "no-store" });
    if (!res.ok && res.status !== 204) throw new Error(`HTTP ${res.status}`);
    const wasOffline = offline;
    offline = "";
    if (res.status === 204) {
      if (wasOffline) render();
      return;
    }
    const next = await res.json();
    version = next.version;
    data = { ...next, definition: next.definition ?? data?.definition, vil: next.vil ?? data?.vil };
    reindex();
    if (data.definition && data.vil) {
      try {
        model = build(data);
      } catch (err) {
        data.vilError = `${data.vilError ?? ""} could not render: ${err.message}`.trim();
      }
    }
    render();
  } catch (err) {
    offline = err.message;
    if (model) render();
  } finally {
    setTimeout(poll, POLL_MS);
  }
}

poll();
