//@ pragma Env QT_DISABLE_HW_TEXTURES_CONVERSION=1
pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Hyprland
import Quickshell.Wayland
import Quickshell.Services.Pam
import Quickshell.Services.Mpris

ShellRoot {
    id: shell
    settings.watchFiles: false

    readonly property bool locking: Quickshell.env("LOCK_MODE") === "lock"
    property string pending: ""
    property bool busy: false
    property var weather: null
    property var tally: ({})
    property var seen: ({})
    property var hides: []
    readonly property var notices: Object.entries(tally)
        .map(([app, count]) => ({ app: app, count: count }))
        .sort((a, b) => b.count - a.count || a.app.localeCompare(b.app))
    readonly property var player: {
        const all = Mpris.players.values
        return all.find(p => p.isPlaying) ?? all.find(p => p.identity === "Spotify") ?? all[0] ?? null
    }

    signal rejected(string text)

    function check(password) {
        if (busy)
            return
        pending = password
        busy = true
        if (!pam.start()) {
            busy = false
            rejected("PAM did not start")
        }
    }

    function accept() {
        console.info("lock pam: success")
        if (!locking) {
            Qt.quit()
            return
        }
        lock.locked = false
        done.start()
    }

    PamContext {
        id: pam
        configDirectory: Quickshell.shellDir + "/pam"
        config: "lock"
        onPamMessage: {
            if (responseRequired) {
                respond(shell.pending)
                shell.pending = ""
            }
        }
        onCompleted: result => {
            shell.busy = false
            console.info("lock pam:", PamResult.toString(result))
            if (result === PamResult.Success)
                shell.accept()
            else
                shell.rejected("")
        }
        onError: error => {
            shell.busy = false
            console.warn("lock pam error:", PamError.toString(error))
            shell.rejected("Authentication error")
        }
    }

    Process {
        id: forecast
        command: [Quickshell.env("HOME") + "/.local/bin/ewwd", "query", "weather"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const w = JSON.parse(text)
                    shell.weather = w.temp === null || w.temp === undefined ? null : w
                } catch (e) {
                    console.warn("lock weather:", e)
                }
            }
        }
    }

    Timer {
        interval: 600000
        running: true
        repeat: true
        onTriggered: forecast.running = true
    }

    function hidden(app, summary, body) {
        return hides.some(rule => [["appname", app], ["summary", summary], ["body", body]]
            .every(([key, value]) => rule[key] === undefined || new RegExp(rule[key]).test(value)))
    }

    Process {
        command: ["dunstctl", "rules", "--json"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    shell.hides = JSON.parse(text).data[0]
                        .map(rule => Object.entries(rule).reduce((out, [key, value]) => Object.assign(out, { [key]: value.data }), {}))
                        .filter(rule => rule.enabled && (rule.skip_display || rule.format === ""))
                } catch (e) {
                    console.warn("lock dunst rules:", e)
                }
            }
        }
    }

    Process {
        command: ["busctl", "--user", "monitor", "--json=short", "--match", "type=method_call,interface=org.freedesktop.Notifications,member=Notify"]
        running: true
        stdout: SplitParser {
            onRead: line => {
                let app, replaces, summary, body, hints
                try {
                    [app, replaces, , summary, body, , hints] = JSON.parse(line).payload.data
                } catch (e) {
                    console.warn("lock notices:", e)
                    return
                }
                if (shell.hidden(app, summary, body))
                    return
                if (replaces !== 0) {
                    const content = JSON.stringify([summary, body])
                    if (shell.seen[replaces] === content)
                        return
                    shell.seen[replaces] = content
                }
                const key = hints?.["desktop-entry"]?.data === "opencode" ? "opencode" : app
                const count = key === "Grok Bot" ? 1 : (shell.tally[key] ?? 0) + 1
                shell.tally = Object.assign({}, shell.tally, { [key]: count })
            }
        }
        onExited: code => console.warn("lock notices: busctl exited", code)
    }

    property var feed: null
    property string feedKey: ""
    property var sessions: []

    FileView {
        id: service
        path: (Quickshell.env("XDG_STATE_HOME") || Quickshell.env("HOME") + "/.local/state") + "/opencode/service.json"
        blockLoading: true
        printErrors: false
    }

    function request(path, done) {
        let registration
        try {
            registration = JSON.parse(service.text())
        } catch (e) {
            done(null)
            return
        }
        const xhr = new XMLHttpRequest()
        xhr.onreadystatechange = () => {
            if (xhr.readyState !== XMLHttpRequest.DONE)
                return
            let body = null
            if (xhr.status === 200) {
                try {
                    body = JSON.parse(xhr.responseText)
                } catch (e) {
                    console.warn("lock opencode:", path, e)
                }
            }
            done(body?.data ?? null)
        }
        xhr.open("GET", registration.url + path)
        if (registration.password)
            xhr.setRequestHeader("Authorization", "Basic " + Qt.btoa(Array.from(unescape(encodeURIComponent("opencode:" + registration.password)))))
        xhr.send()
    }

    function scanAgents() {
        service.reload()
        if (Object.keys(tints).length === 0 && theme.loaded)
            loadTints()
        request("/api/session/active", active => shell.group(Object.keys(active ?? {})))
    }

    function root(id, done) {
        request("/api/session/" + id, info => {
            if (info?.parentID)
                shell.root(info.parentID, done)
            else
                done(info)
        })
    }

    function group(busy) {
        if (busy.length === 0) {
            sessions = []
            return
        }
        const roots = {}
        const entry = info => {
            roots[info.id] = roots[info.id] ?? { id: info.id, title: info.title ?? "", agents: [] }
            return roots[info.id]
        }
        let pending = busy.length
        const finish = () => {
            if (--pending > 0)
                return
            shell.sessions = Object.values(roots)
                .sort((a, b) => b.id.localeCompare(a.id))
                .map(r => ({ title: r.title, subs: r.agents.sort().map(name => shell.tints[name] ?? "") }))
        }
        for (const id of busy) {
            request("/api/session/" + id, info => {
                if (!info?.parentID) {
                    if (info)
                        entry(info)
                    finish()
                    return
                }
                shell.root(info.parentID, top => {
                    if (top)
                        entry(top).agents.push(info.agent ?? "")
                    finish()
                })
            })
        }
    }

    property var tints: ({})

    function loadTints() {
        let palette
        try {
            palette = JSON.parse(theme.text())
        } catch (e) {
            console.warn("lock theme:", e)
            return
        }
        request("/api/agent", agents => {
            const out = {}
            for (const agent of agents ?? []) {
                let value = palette.theme?.[agent.color] ?? agent.color
                if (typeof value === "object")
                    value = value?.dark
                const hex = palette.defs?.[value] ?? value
                if (typeof hex === "string" && hex.startsWith("#"))
                    out[agent.id] = hex
            }
            shell.tints = out
        })
    }

    FileView {
        id: theme
        path: Quickshell.env("HOME") + "/.config/opencode/themes/vagari.json"
        onLoaded: shell.loadTints()
    }

    Timer {
        interval: 5000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: shell.scanAgents()
    }

    Process {
        command: [Quickshell.env("HOME") + "/.local/bin/ewwd", "subscribe", "music"]
        running: true
        stdout: SplitParser {
            onRead: line => {
                let data
                try {
                    data = JSON.parse(line).data
                } catch (e) {
                    console.warn("lock music feed:", e)
                    return
                }
                const canvas = data?.has_canvas ? (data.canvas_path ?? "") : ""
                const art = data?.has_art ? (data.art_path ?? "") : ""
                const key = JSON.stringify([data?.title, canvas, art, data?.queue, data?.history])
                if (key === shell.feedKey)
                    return
                shell.feedKey = key
                shell.feed = { title: data?.title ?? "", canvas: canvas, art: art, queue: data?.queue ?? [], history: data?.history ?? [] }
            }
        }
    }

    Timer {
        id: done
        interval: 300
        onTriggered: Qt.quit()
    }

    readonly property int cover: Number(Quickshell.env("LOCK_COVER_WORKSPACE") ?? 0)
    readonly property var exposures: [
        "openwindow", "closewindow", "movewindow", "movewindowv2", "pin", "changefloatingmode",
        "workspace", "workspacev2", "focusedmon", "focusedmonv2", "moveworkspace", "moveworkspacev2",
        "activespecial", "activespecialv2", "openlayer", "closelayer",
        "monitoradded", "monitoraddedv2", "monitorremoved", "monitorremovedv2", "configreloaded"
    ]
    property string clearOutput: ""
    property string candidate: ""
    property bool dirty: false
    property bool checked: !locking || !(cover > 0)

    function shows(screen) {
        const name = screen?.name ?? ""
        return name !== "" && clearOutput === name
    }

    function inspect() {
        if (checked) {
            clearOutput = ""
            candidate = ""
            settle.stop()
        }
        if (!locking)
            return
        if (scene.running) {
            dirty = true
            return
        }
        scene.running = true
    }

    function judge(text) {
        if (!(cover > 0))
            return ""
        let monitors, clients, layers
        try {
            [monitors, clients, layers] = text.split("\n\n\n").map(part => JSON.parse(part))
        } catch (e) {
            console.warn("lock scene:", e)
            return ""
        }
        if (!Array.isArray(monitors) || !Array.isArray(clients) || !layers)
            return ""
        const focused = monitors.filter(m => m.focused)
        if (focused.length !== 1)
            return ""
        const monitor = focused[0]
        if (monitor.activeWorkspace?.id !== cover || monitor.specialWorkspace?.id !== 0)
            return ""
        if (clients.some(c => c.class !== "GLava" && (c.pinned !== false || c.workspace?.id === cover)))
            return ""
        const levels = layers[monitor.name]?.levels
        if (!levels || ["1", "2", "3"].some(level => levels[level]?.length !== 0))
            return ""
        return monitor.name
    }

    Process {
        id: scene
        command: ["hyprctl", "--batch", "j/monitors; j/clients; j/layers"]
        stdout: StdioCollector { id: sceneOut }
        onExited: code => {
            if (shell.dirty) {
                shell.dirty = false
                running = true
                return
            }
            const name = code === 0 ? shell.judge(sceneOut.text) : ""
            if (!shell.checked) {
                shell.clearOutput = name
                shell.checked = true
                return
            }
            shell.candidate = name
            if (name)
                settle.restart()
        }
    }

    Timer {
        id: settle
        interval: 500
        onTriggered: shell.clearOutput = shell.candidate
    }

    Timer {
        interval: 1000
        running: !shell.checked
        onTriggered: {
            shell.clearOutput = ""
            shell.checked = true
        }
    }

    Connections {
        target: Hyprland
        function onRawEvent(event) {
            if (event.name === "openwindow" && event.parse(4)[2] === "GLava")
                return
            if (shell.exposures.includes(event.name))
                shell.inspect()
        }
        function onFocusedMonitorChanged() { shell.inspect() }
        function onFocusedWorkspaceChanged() { shell.inspect() }
    }

    Component.onCompleted: inspect()

    Process {
        id: bar
        property color tint: "transparent"
        readonly property var monitor: Hyprland.focusedMonitor?.lastIpcObject ?? null
        readonly property int depth: Math.round((monitor?.height ?? 0) * 100 / 1080)
        running: true
        stdinEnabled: true
        command: monitor?.width > 0 && depth > 0 ? [
            "setpriv", "--pdeathsig", "TERM",
            "glava", "-e", "lock_rc.glsl", "--stdin",
            "-r", `setgeometry ${monitor.x} ${monitor.y + monitor.height - depth} ${monitor.width} ${depth}`
        ] : []
        onRunningChanged: paint.restart()
        onTintChanged: paint.restart()
    }

    Timer {
        id: paint
        interval: 100
        onTriggered: if (bar.running) bar.write(`${bar.tint.r},${bar.tint.g},${bar.tint.b},${bar.tint.a}\n`)
    }

    component Screen: Face {
        id: face
        busy: shell.busy
        weather: shell.weather
        notices: shell.notices
        sessions: shell.sessions
        feed: shell.feed
        player: shell.player
        onSubmitted: password => shell.check(password)

        Binding {
            target: bar
            property: "tint"
            value: face.tint
        }

        Connections {
            target: shell
            function onRejected(text) { face.reject(text) }
        }
    }

    Loader {
        active: !shell.locking
        sourceComponent: FloatingWindow {
            implicitWidth: 1920
            implicitHeight: 1080
            color: "black"
            onClosed: Qt.quit()

            Screen { anchors.fill: parent }
        }
    }

    WlSessionLock {
        id: lock
        locked: shell.locking && shell.checked
        onSecureChanged: console.warn(secure ? "lock-secure: acquired" : "lock-secure: lost")

        WlSessionLockSurface {
            id: surface
            color: "transparent"

            Screen {
                anchors.fill: parent
                clear: shell.shows(surface.screen)
            }
        }
    }
}
