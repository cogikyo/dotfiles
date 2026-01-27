# hyprd — Unified Hyprland + System Daemon

A single Go binary replacing all bash scripts for window management and eww status bar data.

## Design Principles

1. **Single daemon** — one process for all Hyprland + eww needs
2. **No temp files** — all state in-memory, clients connect to socket
3. **Event-driven** — subscribe to Hyprland events, react instantly
4. **Incremental adoption** — each module can be ported independently

---

## What Gets Replaced

**Window management scripts (`~/dotfiles/bin/`):**

- `monocle`, `pseudo-master`, `split`, `swap-master`, `ws`, `focus-active`, `layout`, `resize-slave`

**eww scripts (`~/dotfiles/config/eww/bin/`):**

- `workspaces` → already uses socat to Hyprland socket, will be native
- `create_temp_files` → **eliminated entirely**
- `music_info` → integrated as a module
- `timer_info` → integrated as a module (if used)

**Keep separate (system utilities):**

- `brightness`, `network_info`, `pulse_info`, `weather_info`, `ping` — not Hyprland-related

---

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                       hyprd                             │
├──────────────────────────────────────────────────────────────┤
│  State (in-memory, single source of truth)                   │
│  ├── workspace: {current, occupied[], history[]}            │
│  ├── monocle: {addr, origin_ws, position} | null            │
│  ├── pseudo: {addr, slave_index} | null                     │
│  ├── displaced_masters: map[ws]addr                         │
│  ├── split_ratio: "xs" | "default" | "lg"                   │
│  └── music: {status, artist, album, title, volume}          │
├──────────────────────────────────────────────────────────────┤
│  Hyprland IPC                                                │
│  ├── .socket.sock  → commands (dispatch, query clients)     │
│  └── .socket2.sock → events (workspace, openwindow, etc.)   │
├──────────────────────────────────────────────────────────────┤
│  Client Socket: /tmp/hyprd.sock                         │
│  ├── query   → get state: workspace, monocle, music, etc.   │
│  ├── command → monocle, pseudo, split, swap, ws, layout     │
│  └── subscribe → stream events to eww/other clients         │
├──────────────────────────────────────────────────────────────┤
│  Modules                                                     │
│  ├── hypr    → Hyprland socket connection + events          │
│  ├── window  → monocle, pseudo, split, swap, focus          │
│  ├── layout  → session/workspace orchestration              │
│  └── music   → playerctl/Spotify integration                │
└──────────────────────────────────────────────────────────────┘
```

**Key insight:** eww uses `deflisten` with `hyprd subscribe` — direct socket, no temp files.

---

## Implementation Phases

### Phase 0: Scaffolding ✧ START HERE

**Goal:** Minimal daemon that connects to Hyprland and exposes a socket.

```
cmd/hyprd/
├── main.go           # CLI entry (daemon, status commands)
├── hypr/
│   └── socket.go     # Hyprland socket connection
├── daemon/
│   ├── daemon.go     # Lifecycle: start, stop, signal handling
│   └── server.go     # Unix socket server for clients
└── go.mod
```

**Deliverables:**

- [ ] `hyprd` — starts daemon (foreground by default)
- [ ] `hyprd status` — returns "running" / "not running"
- [ ] Connect to Hyprland socket, query clients
- [ ] Basic client socket at `/tmp/hyprd.sock`

**Test:** `hyprd & hyprd status` → "running"

---

### Phase 1: State & Events

**Goal:** Track window state, subscribe to Hyprland events.

**Add:**

```
daemon/
├── state.go          # State struct, mutations
└── events.go         # Hyprland event subscription
```

**State struct:**

```go
type State struct {
    sync.RWMutex
    Workspace     int
    MonocleWindow *MonocleState  // nil if not in monocle
    PseudoMaster  *PseudoState   // nil if not in pseudo
    DisplacedMasters map[int]string
    SplitRatio    string // "xs" | "default" | "lg"
}

type MonocleState struct {
    Address    string
    OriginWS   int
    Position   string // "master" | "0" | "1" | ...
}
```

**Events to handle:**

- `workspace` → update current workspace
- `closewindow` → cleanup monocle/pseudo state if that window closed
- `openwindow` → (future) auto-arrange

**Deliverables:**

- [ ] State struct with thread-safe access
- [ ] Event loop reading from `.socket2.sock`
- [ ] `hyprd status --json` returns full state

---

### Phase 2: Core Commands — monocle

**Goal:** Port `monocle` bash script to Go.

**Add:**

```
commands/
└── monocle.go
```

**Logic (from current bash):**

1. Get active window info
2. If in pseudo-master → restore pseudo first
3. If on WS6 floating → restore to original position
4. Else → save position, float to WS6, change border color

**CLI:**

```bash
hyprd monocle           # toggle monocle on active window
hyprd monocle --restore # force restore any monocle window
```

**Test:** Toggle monocle, verify window moves to WS6, toggle again, verify returns.

---

### Phase 3: Core Commands — split

**Goal:** Port `split` script.

**Add to:** `commands/split.go`

**Logic:**

- Cycle: xs (0.37) → default (0.4942) → lg (0.77) → xs
- Flags: `-x` (xs), `-d` (default), `-l` (lg)

**CLI:**

```bash
hyprd split       # cycle to next
hyprd split -x    # set xs
hyprd split -d    # set default
hyprd split -l    # set lg
```

---

### Phase 4: Core Commands — pseudo

**Goal:** Port `pseudo-master` script.

**Add to:** `commands/pseudo.go`

**Logic:**

1. If floating + has pseudo state → restore to slave position
2. If master → focus right, then pseudo
3. Save slave index, float to cover stack area

**Geometry (3840×2160 monitor):**

- Reserved: top=86, bottom=32, right=85
- Usable height: 2042px

---

### Phase 5: Core Commands — swap, ws, focus

**Add to:** `commands/swap.go`, `commands/ws.go`, `commands/focus.go`

**swap:**

- Track displaced master per workspace
- Toggle: slave→master saves old master, master→slave restores

**ws:**

- Switch workspace
- Auto-focus first window (leftmost = master)

**focus:**

- Smart focus with pseudo-master awareness

---

### Phase 6: eww Integration (Socket-based)

**Goal:** eww connects to daemon instead of tailing temp files.

**Protocol:**

```bash
# Query current state
hyprd query workspace
# → {"current": 3, "occupied": [1,3,4,5]}

hyprd query monocle
# → {"origin_ws": 3, "addr": "0x..."} or null

hyprd query music
# → {"status": "Playing", "artist": "...", "title": "..."}

# Subscribe to changes (keeps connection open, JSON stream)
hyprd subscribe workspace monocle music
# → {"event":"workspace","data":{"current":3,"occupied":[1,3,4,5]}}
# → {"event":"monocle","data":{"origin_ws":3}}
# → {"event":"music","data":{"status":"Playing",...}}
```

**eww integration:**

```yuck
;; Old (tailing temp files)
(deflisten workspace 'tail -F /tmp/eww/workspace')
(defpoll music :interval "1s" `cat /tmp/eww/music.json`)

;; New (socket subscription)
(deflisten hyprd-state `hyprd subscribe workspace monocle music`)
```

**Files to delete after migration:**

- `/tmp/eww/` directory (no longer created)
- `config/eww/bin/create_temp_files`
- `config/eww/bin/workspaces`
- `config/eww/bin/music_info`

---

### Phase 7: Layout/Sessions

**Goal:** Declarative workspace layouts.

**Config:** `~/.config/hyprd/sessions.toml`

```toml
[[session]]
name = "acr"
workspace = 3
project = "acr"
urls = ["localhost:3002"]

[[session]]
name = "dotfiles"
workspace = 4
project = "dotfiles"
urls = ["github.com/cogikyo/dotfiles"]
```

**CLI:**

```bash
hyprd layout acr         # open acr session on WS3
hyprd layout --list      # list available sessions
hyprd layout --current   # show current session
```

---

## File Structure (Final)

```
~/dotfiles/cmd/hyprd/
├── main.go
├── go.mod
├── daemon/
│   ├── daemon.go       # lifecycle, signal handling
│   ├── server.go       # client socket (/tmp/hyprd.sock)
│   └── state.go        # unified state management
├── hypr/
│   ├── socket.go       # Hyprland IPC connection
│   ├── events.go       # Event subscription (.socket2.sock)
│   ├── client.go       # Window queries (clients, activewindow)
│   └── dispatch.go     # Command execution (dispatch, keyword)
├── window/
│   ├── monocle.go
│   ├── pseudo.go
│   ├── split.go
│   ├── swap.go
│   ├── ws.go
│   └── focus.go
├── layout/
│   ├── layout.go       # Session orchestration
│   └── sessions.go     # TOML config parsing
└── modules/
    └── music.go        # Spotify/playerctl integration
```

---

## Migration Checklist

After each phase, update configs incrementally:

**Window management (binds.conf):**

| Bash Script     | Go Command                     | Phase |
| --------------- | ------------------------------ | ----- |
| `monocle`       | `hyprd monocle`                | 2     |
| `split`         | `hyprd split`                  | 3     |
| `pseudo-master` | `hyprd pseudo`                 | 4     |
| `swap-master`   | `hyprd swap`                   | 5     |
| `ws`            | `hyprd ws`                     | 5     |
| `focus-active`  | `hyprd focus`                  | 5     |
| `layout`        | `hyprd layout`                 | 7     |
| `resize-slave`  | (integrated into pseudo/split) | 4     |

**eww scripts (eww.yuck):**

| Bash Script         | Go Command                  | Phase |
| ------------------- | --------------------------- | ----- |
| `workspaces`        | `hyprd subscribe workspace` | 6     |
| `music_info`        | `hyprd subscribe music`     | 6     |
| `create_temp_files` | **deleted** (no temp files) | 6     |

---

## Current Progress

- [ ] Phase 0: Scaffolding — daemon lifecycle, Hyprland connection, client socket
- [ ] Phase 1: State & Events — unified state, event subscription
- [ ] Phase 2: monocle
- [ ] Phase 3: split
- [ ] Phase 4: pseudo
- [ ] Phase 5: swap, ws, focus
- [ ] Phase 6: eww integration — query/subscribe protocol, music module
- [ ] Phase 7: layout/sessions

---

## Notes

**No temp files.** All state lives in daemon memory. Clients connect to socket.

**Hyprland sockets:**

- `.socket.sock` — commands (dispatch, keyword, clients query)
- `.socket2.sock` — event subscription (workspace changes, window open/close)

**Dependencies:** Pure Go stdlib. No external deps.

**Startup:**

```conf
# hyprland.conf
exec-once = hyprd  # replaces: workspaces, create_temp_files, music_info get_json
```

**CLI pattern:**

```bash
hyprd                    # start daemon (foreground)
hyprd status             # check if running
hyprd monocle            # window command (talks to daemon)
hyprd query workspace    # query state
hyprd subscribe ws music # stream events (for eww)
```
