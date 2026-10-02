import QtQuick
import QtQuick.Controls
import QtMultimedia

Rectangle {
    width: 1920
    height: 1080
    color: "black"

    Image {
        anchors.fill: parent
        source: "file:///usr/share/backgrounds/dotfiles/dna-still.png"
        fillMode: Image.PreserveAspectCrop
    }

    MediaPlayer {
        id: player
        source: "file:///usr/share/backgrounds/dotfiles/dna.mp4"
        loops: MediaPlayer.Infinite
        videoOutput: video
        Component.onCompleted: play()
    }

    VideoOutput {
        id: video
        anchors.fill: parent
        fillMode: VideoOutput.PreserveAspectCrop
    }

    Column {
        anchors.centerIn: parent
        width: 320
        spacing: 12

        TextField {
            id: user
            width: parent.width
            placeholderText: "user"
            text: userModel.lastUser
            onAccepted: password.forceActiveFocus()
        }

        TextField {
            id: password
            width: parent.width
            placeholderText: "password"
            echoMode: TextInput.Password
            onAccepted: sddm.login(user.text, text, sessionModel.lastIndex)
        }

        Text {
            id: message
            width: parent.width
            color: "white"
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.Wrap
        }
    }

    Connections {
        target: sddm

        function onLoginFailed() {
            password.clear()
            message.text = "Login failed"
            password.forceActiveFocus()
        }

        function onInformationMessage(text) {
            message.text = text
        }
    }

    Component.onCompleted: (user.text ? password : user).forceActiveFocus()
}
