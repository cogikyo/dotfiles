pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Quickshell.Io
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
    property int notices: 0
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
        Quickshell.execDetached(["dunstctl", "set-paused", "false"])
        lock.locked = false
        done.start()
    }

    Component.onCompleted: {
        if (locking)
            Quickshell.execDetached(["dunstctl", "set-paused", "true"])
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

    Process {
        id: count
        command: ["dunstctl", "count"]
        stdout: StdioCollector {
            onStreamFinished: {
                const field = name => Number(text.match(new RegExp(name + ":\\s*(\\d+)"))?.[1] ?? 0)
                shell.notices = field("Waiting") + field("Currently displayed")
            }
        }
    }

    Timer {
        interval: 3000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: count.running = true
    }

    property var feed: null
    property string feedKey: ""
    property int agents: 0

    function request(path, directory, done) {
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
            done(body)
        }
        xhr.open("GET", "http://127.0.0.1:4096" + path)
        if (directory)
            xhr.setRequestHeader("x-opencode-directory", directory)
        xhr.send()
    }

    function countAgents() {
        request("/project", "", projects => {
            if (!projects) {
                shell.agents = 0
                return
            }
            const dirs = [...new Set(projects.reduce((all, p) => all.concat(p.worktree, p.sandboxes ?? []), []).filter(d => d && d !== "/"))]
            if (dirs.length === 0) {
                shell.agents = 0
                return
            }
            let pending = dirs.length
            let busy = 0
            for (const dir of dirs) {
                request("/session/status", dir, statuses => {
                    busy += Object.values(statuses ?? {}).filter(s => s.type === "busy" || s.type === "retry").length
                    if (--pending === 0)
                        shell.agents = busy
                })
            }
        })
    }

    Timer {
        interval: 5000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: shell.countAgents()
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
                const key = JSON.stringify([data?.title, canvas, data?.queue, data?.history])
                if (key === shell.feedKey)
                    return
                shell.feedKey = key
                shell.feed = { title: data?.title ?? "", canvas: canvas, queue: data?.queue ?? [], history: data?.history ?? [] }
            }
        }
    }

    Timer {
        id: done
        interval: 300
        onTriggered: Qt.quit()
    }

    component Screen: Face {
        id: face
        busy: shell.busy
        weather: shell.weather
        notices: shell.notices
        agents: shell.agents
        feed: shell.feed
        player: shell.player
        onSubmitted: password => shell.check(password)

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
        locked: shell.locking
        onSecureChanged: console.info("lock secure:", secure)

        WlSessionLockSurface {
            color: "black"

            Screen { anchors.fill: parent }
        }
    }
}
