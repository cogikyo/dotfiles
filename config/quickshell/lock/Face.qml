pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Effects

Rectangle {
    id: root
    color: root.clear ? "transparent" : "black"

    readonly property color navy: "#222536"
    readonly property color lavender: "#aeb9f8"
    readonly property color steel: "#7690b9"
    readonly property color blue: "#7492ef"
    readonly property color peach: "#f2a170"
    readonly property color pink: "#e36cb8"
    readonly property color shade: "#131626"
    readonly property color night: "#181b2c"
    readonly property string face: "Adwaita Sans"
    readonly property real s: height / 1080
    readonly property real edge: 96 * s

    property bool clear: false
    property bool busy: false
    property var weather: null
    property var notices: []
    property var sessions: []
    property real breath: 1
    property var player: null
    property var feed: null
    property string message: ""
    property date now: new Date()

    signal submitted(string password)

    function reject(text) {
        input.clear()
        message = text
        helix.fail()
        input.forceActiveFocus()
    }

    Image {
        anchors.fill: parent
        visible: !root.clear
        source: "file:///usr/share/backgrounds/dotfiles/dna-still.png"
        fillMode: Image.PreserveAspectCrop
    }

    MouseArea {
        anchors.fill: parent
        onClicked: input.forceActiveFocus()
    }

    Timer {
        running: true
        repeat: true
        interval: 1000
        onTriggered: root.now = new Date()
    }

    component Shadowed: MultiEffect {
        shadowEnabled: true
        shadowColor: root.shade
        shadowBlur: 0.5
    }

    component Label: Text {
        font.family: root.face
        font.pixelSize: 30 * root.s
        color: root.lavender
    }

    SequentialAnimation on breath {
        running: root.sessions.length > 0
        loops: Animation.Infinite
        NumberAnimation { to: 0.4; duration: 1100; easing.type: Easing.InOutSine }
        NumberAnimation { to: 1; duration: 1100; easing.type: Easing.InOutSine }
    }

    component Glyph: Text {
        property int code
        width: 32 * root.s
        horizontalAlignment: Text.AlignHCenter
        text: String.fromCodePoint(code)
        font.family: "Symbols Nerd Font"
        font.pixelSize: 28 * root.s
    }

    Column {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.topMargin: 64 * root.s
        anchors.leftMargin: root.edge
        spacing: 4 * root.s
        layer.enabled: true
        layer.effect: Shadowed {}

        Text {
            text: Qt.formatTime(root.now, "HH:mm")
            font.family: "Archivo"
            font.weight: Font.Medium
            font.variableAxes: ({ "wght": 500 })
            font.pixelSize: 150 * root.s
            color: root.blue
        }

        Row {
            spacing: 16 * root.s

            Label { text: Qt.formatDate(root.now, "dddd, MMMM d") }

            Label {
                visible: root.weather !== null
                text: "·"
                color: root.steel
            }

            Label {
                visible: root.weather !== null
                anchors.verticalCenter: parent.verticalCenter
                text: (root.weather?.icon ?? "").trim()
                font.family: "Symbols Nerd Font"
                font.pixelSize: 28 * root.s
            }

            Label {
                visible: root.weather !== null
                leftPadding: -6 * root.s
                text: `${root.weather?.temp ?? ""}°`
            }
        }

    }

    Column {
        anchors.top: parent.top
        anchors.left: parent.horizontalCenter
        anchors.topMargin: 88 * root.s
        anchors.leftMargin: -380 * root.s
        spacing: 12 * root.s
        layer.enabled: true
        layer.effect: Shadowed {}

        Row {
            visible: root.notices.length > 0
            spacing: 14 * root.s

            Glyph {
                anchors.verticalCenter: parent.verticalCenter
                code: 0xF009A
                color: root.peach
            }

            Repeater {
                model: root.notices

                Row {
                    id: group
                    required property var modelData
                    anchors.verticalCenter: parent.verticalCenter
                    rightPadding: 12 * root.s
                    spacing: 8 * root.s

                    Label {
                        text: group.modelData.count
                        color: root.peach
                        font.pixelSize: 26 * root.s
                    }

                    Label {
                        text: group.modelData.app
                        font.pixelSize: 26 * root.s
                    }
                }
            }
        }

        Repeater {
            model: root.sessions

            Row {
                id: session
                required property var modelData
                spacing: 14 * root.s

                Row {
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 4 * root.s

                    Glyph {
                        anchors.verticalCenter: parent.verticalCenter
                        code: 0xF0BC9
                        color: root.blue
                        opacity: root.breath
                    }

                    Repeater {
                        model: session.modelData.subs

                        Glyph {
                            required property string modelData
                            anchors.verticalCenter: parent.verticalCenter
                            width: 20 * root.s
                            code: 0xF0BC9
                            color: modelData || root.steel
                            opacity: root.breath
                            font.pixelSize: 18 * root.s
                        }
                    }
                }

                Label {
                    anchors.verticalCenter: parent.verticalCenter
                    width: Math.min(implicitWidth, 720 * root.s)
                    elide: Text.ElideRight
                    text: session.modelData.title
                    color: root.blue
                    font.pixelSize: 26 * root.s
                }
            }
        }
    }

    Helix {
        id: helix
        z: 1
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.rightMargin: root.edge * 0.75
        anchors.bottomMargin: 42 * root.s
        width: parent.width * 0.58
        height: 160 * root.s
        active: input.length > 0
        checking: root.busy
        pale: root.lavender
        strand: root.blue
    }

    Music {
        anchors.right: parent.right
        anchors.top: parent.top
        room: helix.y - 36 * root.s
        floor: root.height
        s: root.s
        player: root.player
        feed: root.feed
        text: root.lavender
        muted: root.steel
        accent: root.blue
        active: root.peach
        well: root.navy
        night: root.night
        shade: root.shade
        face: root.face
    }

    Text {
        anchors.horizontalCenter: helix.horizontalCenter
        anchors.bottom: helix.top
        anchors.bottomMargin: 8 * root.s
        visible: root.message !== ""
        text: root.message
        font.family: root.face
        font.pixelSize: 20 * root.s
        color: root.pink
    }

    TextInput {
        id: input
        property int last: 0
        width: 1
        height: 1
        opacity: 0
        echoMode: TextInput.Password
        enabled: !root.busy
        onTextEdited: {
            root.message = ""
            helix.pulse(length < last)
            last = length
        }
        onTextChanged: if (length === 0) last = 0
        onAccepted: if (text) root.submitted(text)
        Keys.onEscapePressed: clear()
        Component.onCompleted: forceActiveFocus()
    }
}
