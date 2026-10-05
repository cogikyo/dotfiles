pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Effects
import QtQuick.Shapes
import QtMultimedia
import Quickshell.Widgets

Item {
    id: music

    required property real s
    required property var player
    property var feed: null
    property real room: 0
    property color text: "#aeb9f8"
    property color muted: "#7690b9"
    property color accent: "#7492ef"
    property color active: "#f2a170"
    property color well: "#222536"
    property color night: "#0b0d1a"
    property color shade: "#131626"
    property string face: "Adwaita Sans"
    property int skips: 0

    readonly property bool playing: player?.isPlaying ?? false
    readonly property string title: player?.trackTitle ?? ""
    readonly property string artist: player?.trackArtist ?? ""
    readonly property string art: player?.trackArtUrl ?? ""
    readonly property string key: [title, artist, art].join("\n")
    readonly property string canvas: feed?.canvas && feed.title === title ? "file://" + feed.canvas : ""
    readonly property real big: 150 * s
    readonly property real small: 46 * s
    readonly property var fades: [0, 0.38, 0.6, 1, 0.9, 0.68, 0]
    readonly property real progress: player?.lengthSupported && player.length > 0 ? Math.min(1, player.position / player.length) : 0
    property bool flipped: false
    property bool waiting: false

    visible: title !== ""
    width: 600 * s
    height: room

    onKeyChanged: {
        settle.restart()
        debounce.restart()
    }
    onFeedChanged: debounce.restart()
    Component.onCompleted: {
        settle.restart()
        debounce.restart()
    }

    function norm(text) {
        return (text ?? "").trim().toLowerCase()
    }

    function find(key) {
        for (let i = 0; i < strip.count; i++)
            if (strip.get(i).key === key)
                return i
        return -1
    }

    function sweep() {
        for (let i = strip.count - 1; i >= 0; i--)
            if (strip.get(i).done)
                strip.remove(i)
    }

    function reconcile(force) {
        const current = norm(title)
        const history = feed?.history ?? []
        const queue = feed?.queue ?? []
        const stale = feed?.title !== title || norm(queue[0]?.title) === current || norm(history[0]?.title) === current
        if (current !== "" && stale && !force) {
            if (!holdout.running)
                holdout.start()
            return
        }
        holdout.stop()

        const seen = new Set([current])
        const take = (list, n) => {
            const out = []
            for (const track of list) {
                if (out.length === n)
                    break
                const k = norm(track.title)
                if (k === "" || seen.has(k))
                    continue
                seen.add(k)
                out.push({ key: k, art: track.art_url ?? "" })
            }
            return out
        }
        const before = current === "" ? [] : take(history, 2).reverse()
        const after = current === "" ? [] : take(queue, 2)
        const target = current === "" ? [] : before.concat([{ key: current, art: art }], after)
        const keep = new Set(target.map(t => t.key))
        const fresh = strip.count === 0

        for (let i = 0; i < strip.count; i++) {
            const row = strip.get(i)
            if (keep.has(row.key) || row.gone)
                continue
            strip.setProperty(i, "gone", true)
            strip.setProperty(i, "slot", Math.sign(row.slot) * 3)
        }

        target.forEach((t, i) => {
            const slot = i - before.length
            const at = find(t.key)
            if (at < 0) {
                strip.append({ key: t.key, art: t.art, slot: slot, entry: fresh ? slot : slot + Math.sign(slot), gone: false, done: false })
                return
            }
            strip.setProperty(at, "slot", slot)
            strip.setProperty(at, "gone", false)
            strip.setProperty(at, "done", false)
            if (strip.get(at).art === "" && t.art !== "")
                strip.setProperty(at, "art", t.art)
        })
    }

    function offset(p) {
        const anchor = k => k === 0 ? 0 : big / 2 + 18 * s + small / 2 + (k - 1) * (small + 10 * s)
        const a = Math.abs(p)
        const lo = Math.floor(a)
        const v = anchor(lo) + (anchor(lo + 1) - anchor(lo)) * (a - lo)
        return p < 0 ? -v : v
    }

    function extent(p) {
        return small + (big - small) * Math.max(0, 1 - Math.abs(p))
    }

    function fade(p) {
        const q = Math.max(0, Math.min(6, p + 3))
        const lo = Math.floor(q)
        const hi = Math.min(6, lo + 1)
        return fades[lo] + (fades[hi] - fades[lo]) * (q - lo)
    }

    function jump(n) {
        if (!player || n === 0)
            return
        skips = n < 0 && player.position > 3 ? n - 1 : n
        step()
    }

    function step() {
        if (!player || skips === 0) {
            skips = 0
            return
        }
        if (skips > 0) {
            player.next()
            skips--
        } else {
            player.previous()
            skips++
        }
        if (skips !== 0)
            stepper.restart()
    }

    Timer {
        id: stepper
        interval: 220
        onTriggered: music.step()
    }

    Timer {
        interval: 500
        repeat: true
        running: music.visible && music.playing
        onTriggered: music.player.positionChanged()
    }

    Timer {
        id: debounce
        interval: 200
        onTriggered: music.reconcile(false)
    }

    Timer {
        id: holdout
        interval: 2500
        onTriggered: music.reconcile(true)
    }

    HoverHandler {
        id: hover
    }

    function stage() {
        const back = flipped ? first : second
        if (back.opacity > 0) {
            waiting = true
            return
        }
        waiting = false
        back.load()
    }

    Timer {
        id: settle
        interval: 160
        onTriggered: music.stage()
    }

    component Cover: ClippingRectangle {
        id: cover
        property string source
        readonly property alias status: picture.status

        radius: width * 0.12
        color: music.well
        border.width: Math.max(1, music.s)
        border.color: Qt.rgba(music.text.r, music.text.g, music.text.b, 0.14)

        Image {
            id: picture
            anchors.fill: parent
            source: cover.source
            sourceSize: Qt.size(music.big * 2, music.big * 2)
            fillMode: Image.PreserveAspectCrop
            asynchronous: true
            mipmap: true
            opacity: status === Image.Ready ? 1 : 0
            Behavior on opacity { NumberAnimation { duration: 300 } }
        }
    }

    component Card: Item {
        id: card
        property bool shown: false
        property string key
        property string title
        property string artist
        property string art
        property string canvas
        property bool rolling: false
        readonly property bool live: key !== "" && key === music.key
        signal ready

        width: music.width
        height: music.height
        opacity: shown ? 1 : 0
        visible: opacity > 0
        Behavior on opacity { NumberAnimation { duration: 600; easing.type: Easing.InOutQuad } }

        onOpacityChanged: if (opacity === 0 && music.waiting) music.stage()
        onVisibleChanged: clip.sync()
        onCanvasChanged: rolling = false

        function load() {
            key = music.key
            title = music.title
            artist = music.artist
            art = music.art
            canvas = music.canvas
            patience.restart()
            check()
        }

        function check() {
            if (shown || !live)
                return
            if (art === "" || still.status === Image.Ready || still.status === Image.Error)
                ready()
        }

        Timer {
            id: patience
            interval: 1500
            onTriggered: if (!card.shown && card.live) card.ready()
        }

        Connections {
            target: music
            enabled: card.live
            function onCanvasChanged() { card.canvas = music.canvas }
            function onPlayingChanged() { clip.sync() }
        }

        Connections {
            target: reel.videoSink
            function onVideoFrameChanged() {
                if (card.canvas !== "")
                    card.rolling = true
            }
        }

        MediaPlayer {
            id: clip
            source: card.canvas
            loops: MediaPlayer.Infinite
            videoOutput: reel
            onSourceChanged: sync()

            function sync() {
                if (source.toString() !== "" && card.visible && music.playing)
                    play()
                else
                    pause()
            }
        }

        Item {
            anchors.fill: parent
            layer.enabled: card.visible
            layer.effect: ShaderEffect {
                property size area: Qt.size(width, height)
                property vector4d edges: Qt.vector4d(110 * music.s, 0, 0, 90 * music.s)
                fragmentShader: "feather.frag.qsb"
            }

            Item {
                id: scene
                anchors.fill: parent

                Image {
                    id: still
                    anchors.fill: parent
                    visible: reel.opacity < 1
                    source: card.art
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    mipmap: true
                    onStatusChanged: card.check()
                }

                Rectangle {
                    anchors.fill: parent
                    color: Qt.rgba(music.night.r, music.night.g, music.night.b, 0.4)
                }

                VideoOutput {
                    id: reel
                    anchors.fill: parent
                    fillMode: VideoOutput.PreserveAspectCrop
                    opacity: card.rolling ? 1 : 0
                    Behavior on opacity { NumberAnimation { duration: 500; easing.type: Easing.InOutQuad } }
                }
            }

            Rectangle {
                width: parent.width
                height: parent.height * 0.25
                gradient: Gradient {
                    GradientStop { position: 0; color: Qt.rgba(music.night.r, music.night.g, music.night.b, 0.5) }
                    GradientStop { position: 1; color: "transparent" }
                }
            }

            Rectangle {
                anchors.bottom: parent.bottom
                width: parent.width
                height: parent.height * 0.45
                gradient: Gradient {
                    GradientStop { position: 0; color: "transparent" }
                    GradientStop { position: 1; color: Qt.rgba(music.night.r, music.night.g, music.night.b, 0.8) }
                }
            }
        }

        Column {
            id: words
            x: 72 * music.s
            y: 44 * music.s
            width: parent.width - 144 * music.s
            spacing: 6 * music.s
            layer.enabled: card.visible
            layer.effect: MultiEffect {
                shadowEnabled: true
                shadowColor: music.shade
                shadowOpacity: 0.9
                shadowBlur: 1
                blurMax: Math.round(40 * music.s)
            }

            Caption {
                text: card.title
                wrapMode: Text.WordWrap
                maximumLineCount: 2
                font.weight: Font.DemiBold
                font.pixelSize: 34 * music.s
            }

            Caption {
                text: card.artist
                font.weight: Font.Medium
                font.pixelSize: 24 * music.s
            }
        }

    }

    component Caption: Text {
        width: parent.width
        elide: Text.ElideRight
        font.family: music.face
        color: music.accent
        layer.enabled: visible
        layer.effect: MultiEffect {
            shadowEnabled: true
            shadowColor: music.shade
            shadowBlur: 0.25
            blurMax: Math.round(8 * music.s)
            shadowVerticalOffset: music.s
        }
    }

    component Skip: Text {
        id: skip
        property bool forward
        readonly property bool able: forward ? music.player?.canGoNext ?? false : music.player?.canGoPrevious ?? false

        x: forward ? (deck.width + music.big) / 2 + 12 * music.s : (deck.width - music.big) / 2 - 12 * music.s - width
        y: (deck.height + (music.small + music.big) / 2 - height) / 2
        text: String.fromCodePoint(forward ? 0xF04AD : 0xF04AE)
        font.family: "Symbols Nerd Font"
        font.pixelSize: 32 * music.s
        color: press.containsMouse ? music.active : music.muted
        opacity: hover.hovered && able ? 1 : 0
        visible: opacity > 0
        Behavior on opacity { NumberAnimation { duration: 200 } }
        Behavior on color { ColorAnimation { duration: 120 } }

        MouseArea {
            id: press
            anchors.fill: parent
            anchors.margins: -8 * music.s
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onClicked: skip.forward ? music.player.next() : music.player.previous()
        }
    }

    Card {
        id: first
        shown: key !== "" && !music.flipped
        onReady: music.flipped = false
    }

    Card {
        id: second
        shown: key !== "" && music.flipped
        onReady: music.flipped = true
    }

    Item {
        id: deck
        x: 72 * music.s
        y: parent.height - 56 * music.s - height
        width: parent.width - 144 * music.s
        height: music.big

        Repeater {
            model: ListModel { id: strip }

            Cover {
                id: tile
                required property int index
                required property string key
                required property string art
                required property real slot
                required property real entry
                required property bool gone
                property real place: entry
                property real presence: 0
                property real lift: toggle.containsMouse && !focused ? 1 : 0
                readonly property bool focused: slot === 0 && !gone
                readonly property real level: music.fade(place)

                x: deck.width / 2 + music.offset(place) - width / 2
                y: (deck.height - height) / 2
                z: 3 - Math.abs(place)
                width: music.extent(place)
                height: width
                source: art
                opacity: presence * (level + (1 - level) * lift)

                Behavior on place { NumberAnimation { duration: 560; easing.type: Easing.InOutCubic } }
                Behavior on presence { NumberAnimation { duration: 420; easing.type: Easing.InOutQuad } }
                Behavior on lift { NumberAnimation { duration: 150 } }

                onSlotChanged: place = slot
                onGoneChanged: presence = gone ? 0 : 1
                onPresenceChanged: {
                    if (!gone || presence > 0)
                        return
                    strip.setProperty(index, "done", true)
                    Qt.callLater(music.sweep)
                }
                Component.onCompleted: {
                    place = slot
                    presence = gone ? 0 : 1
                }

                Shape {
                    id: ring
                    property real line: (toggle.containsMouse ? 11 : 7) * music.s
                    readonly property real o: line / 2
                    readonly property real w: tile.width - line
                    readonly property real h: tile.height - line
                    readonly property real r: Math.max(0, tile.radius - o)
                    readonly property real span: 2 * (w + h) - 8 * r + 2 * Math.PI * r
                    property real shown: music.progress
                    property color tone: toggle.containsMouse ? music.active : music.accent

                    anchors.fill: parent
                    z: 1
                    opacity: tile.focused ? 1 : 0
                    visible: opacity > 0
                    layer.enabled: visible
                    layer.samples: 4
                    Behavior on opacity { NumberAnimation { duration: 300 } }
                    Behavior on line { NumberAnimation { duration: 160; easing.type: Easing.OutCubic } }
                    Behavior on tone { ColorAnimation { duration: 160 } }
                    Behavior on shown {
                        enabled: ring.opacity === 1
                        NumberAnimation { duration: 500 }
                    }

                    ShapePath {
                        strokeColor: Qt.rgba(ring.tone.r, ring.tone.g, ring.tone.b, 0.2)
                        strokeWidth: ring.line
                        fillColor: "transparent"

                        PathRectangle {
                            x: ring.o
                            y: ring.o
                            width: ring.w
                            height: ring.h
                            radius: ring.r
                        }
                    }

                    ShapePath {
                        strokeColor: ring.shown > 0.002 ? ring.tone : "transparent"
                        strokeWidth: ring.line
                        fillColor: "transparent"
                        capStyle: ShapePath.RoundCap
                        strokeStyle: ShapePath.DashLine
                        dashPattern: [ring.shown * ring.span / ring.line, ring.span / ring.line + 1]
                        startX: ring.o + ring.w / 2
                        startY: ring.o

                        PathLine { x: ring.o + ring.w - ring.r; y: ring.o }
                        PathArc { x: ring.o + ring.w; y: ring.o + ring.r; radiusX: ring.r; radiusY: ring.r }
                        PathLine { x: ring.o + ring.w; y: ring.o + ring.h - ring.r }
                        PathArc { x: ring.o + ring.w - ring.r; y: ring.o + ring.h; radiusX: ring.r; radiusY: ring.r }
                        PathLine { x: ring.o + ring.r; y: ring.o + ring.h }
                        PathArc { x: ring.o; y: ring.o + ring.h - ring.r; radiusX: ring.r; radiusY: ring.r }
                        PathLine { x: ring.o; y: ring.o + ring.r }
                        PathArc { x: ring.o + ring.r; y: ring.o; radiusX: ring.r; radiusY: ring.r }
                        PathLine { x: ring.o + ring.w / 2; y: ring.o }
                    }
                }

                Rectangle {
                    anchors.fill: parent
                    color: Qt.rgba(music.night.r, music.night.g, music.night.b, 0.5)
                    opacity: toggle.containsMouse && tile.focused ? 1 : 0
                    visible: opacity > 0
                    Behavior on opacity { NumberAnimation { duration: 160 } }

                    Text {
                        anchors.centerIn: parent
                        text: String.fromCodePoint(music.playing ? 0xF03E4 : 0xF040A)
                        font.family: "Symbols Nerd Font"
                        font.pixelSize: 48 * music.s
                        color: music.active
                    }
                }

                MouseArea {
                    id: toggle
                    anchors.fill: parent
                    enabled: !tile.gone && music.player !== null
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: tile.slot === 0 ? music.player.togglePlaying() : music.jump(tile.slot)
                }
            }
        }

        Skip { forward: false }
        Skip { forward: true }
    }
}
