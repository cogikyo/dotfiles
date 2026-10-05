pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls.Basic
import QtQuick.Effects
import QtMultimedia

Rectangle {
    id: root
    width: 1920
    height: 1080
    color: "black"

    readonly property color navy: "#222536"
    readonly property color lavender: "#aeb9f8"
    readonly property color steel: "#7690b9"
    readonly property color glacier: "#3f4578"
    readonly property color blue: "#7492ef"
    readonly property color ruby: "#f08898"
    readonly property color shade: "#131626"
    readonly property string face: "Adwaita Sans"

    property bool failed: false
    property date now: new Date()

    function submit() {
        if (!user.text) {
            user.forceActiveFocus()
            return
        }
        if (!password.text) {
            password.forceActiveFocus()
            return
        }
        sddm.login(user.text, password.text, sessionModel.lastIndex)
    }

    function say(text, failure) {
        message.text = text
        failed = failure
    }

    Image {
        anchors.fill: parent
        source: "file:///usr/share/backgrounds/dotfiles/dna-still.png"
        fillMode: Image.PreserveAspectCrop
    }

    MediaPlayer {
        id: player
        source: "file:///usr/share/backgrounds/dotfiles/dna.webm"
        loops: MediaPlayer.Infinite
        videoOutput: video
        Component.onCompleted: play()
    }

    VideoOutput {
        id: video
        anchors.fill: parent
        fillMode: VideoOutput.PreserveAspectCrop
    }

    FontLoader {
        id: archivo
        source: "fonts/Archivo.ttf"
    }

    Timer {
        running: true
        interval: 60000 - root.now.getSeconds() * 1000 - root.now.getMilliseconds()
        onTriggered: {
            root.now = new Date()
            restart()
        }
    }

    component Shadowed: MultiEffect {
        shadowEnabled: true
        shadowColor: root.shade
        shadowBlur: 0.4
    }

    Text {
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.verticalCenter: parent.verticalCenter
        anchors.verticalCenterOffset: -380
        text: Qt.formatTime(root.now, "HH:mm")
        font.family: archivo.name
        font.weight: Font.Medium
        font.variableAxes: ({ "wght": 500 })
        font.pixelSize: 160
        color: root.blue
        layer.enabled: true
        layer.effect: Shadowed {}
    }

    Text {
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.verticalCenter: parent.verticalCenter
        anchors.verticalCenterOffset: -260
        text: Qt.formatDate(root.now, "dddd, MMMM d")
        font.family: root.face
        font.pixelSize: 32
        color: root.lavender
        layer.enabled: true
        layer.effect: Shadowed {}
    }

    component Field: TextField {
        width: 320
        height: 44
        leftPadding: 14
        rightPadding: 14
        horizontalAlignment: TextInput.AlignHCenter
        font.family: root.face
        font.pixelSize: 16
        color: root.lavender
        placeholderTextColor: root.steel
        selectionColor: root.blue
        selectedTextColor: root.navy
        onTextEdited: root.say("", false)
        onAccepted: root.submit()

        background: Rectangle {
            radius: 12
            color: root.navy
            border.width: 2
            border.color: parent.activeFocus ? root.blue : root.glacier
        }
    }

    Field {
        id: user
        anchors.bottom: password.top
        anchors.bottomMargin: 12
        anchors.horizontalCenter: parent.horizontalCenter
        placeholderText: "user"
        text: userModel.lastUser
    }

    Field {
        id: password
        anchors.centerIn: parent
        placeholderText: "password"
        echoMode: TextInput.Password
    }

    Rectangle {
        anchors.top: password.bottom
        anchors.topMargin: 12
        anchors.horizontalCenter: parent.horizontalCenter
        width: password.width
        height: message.implicitHeight + 20
        radius: 12
        color: root.navy
        visible: message.text !== ""

        Text {
            id: message
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.margins: 10
            font.family: root.face
            font.pixelSize: 15
            color: root.failed ? root.ruby : root.lavender
            horizontalAlignment: Text.AlignHCenter
            verticalAlignment: Text.AlignVCenter
            wrapMode: Text.Wrap
        }
    }

    Connections {
        target: sddm

        function onLoginFailed() {
            password.clear()
            root.say("Sign-in failed. Try again.", true)
            password.forceActiveFocus()
        }

        function onInformationMessage(text) {
            root.say(text, false)
        }
    }

    Component.onCompleted: (user.text ? password : user).forceActiveFocus()
}
