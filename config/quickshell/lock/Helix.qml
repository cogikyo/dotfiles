import QtQuick

ShaderEffect {
    id: helix

    property bool active: false
    property bool checking: false
    property bool failing: false

    property color pale: "#aeb9f8"
    property color strand: "#7492ef"
    property color deep: "#3f4578"
    property color alarm: "#e36cb8"
    property color adenine: "#95cb79"
    property color thymine: "#f36978"
    property color guanine: "#f2a170"
    property color cytosine: "#7cc5ef"

    property real phase: 0
    property real time: 0
    property real boost: 0
    property real busy: checking ? 1 : 0
    property real busyStart: 0
    property real failStart: -100
    readonly property real aspect: width / height
    readonly property real rung: 4.2 / 10.5

    property vector4d s0: Qt.vector4d(0, -100, 0, 0)
    property vector4d s1: Qt.vector4d(0, -100, 0, 0)
    property vector4d s2: Qt.vector4d(0, -100, 0, 0)
    property vector4d s3: Qt.vector4d(0, -100, 0, 0)
    property vector4d s4: Qt.vector4d(0, -100, 0, 0)
    property vector4d s5: Qt.vector4d(0, -100, 0, 0)
    property vector4d s6: Qt.vector4d(0, -100, 0, 0)
    property vector4d s7: Qt.vector4d(0, -100, 0, 0)
    property int slot: 0
    property real last: 0

    function pulse(erasing) {
        const reach = Math.floor(aspect * 0.55 / rung)
        let index = Math.floor(Math.random() * (2 * reach + 1)) - reach
        if (index === last)
            index = index === reach ? index - 1 : index + 1
        last = index
        const value = Qt.vector4d(index, time, Math.random() < 0.5 ? 0 : 1, erasing ? 1 : 0)
        switch (slot) {
        case 0: s0 = value; break
        case 1: s1 = value; break
        case 2: s2 = value; break
        case 3: s3 = value; break
        case 4: s4 = value; break
        case 5: s5 = value; break
        case 6: s6 = value; break
        case 7: s7 = value; break
        }
        slot = (slot + 1) % 8
        boost = Math.min(boost + 1.4, 5)
    }

    function fail() {
        failStart = time
        failing = true
        failed.restart()
        shake.restart()
    }

    onCheckingChanged: if (checking) busyStart = time

    opacity: active || checking || failing ? 1 : 0
    visible: opacity > 0
    Behavior on opacity { NumberAnimation { duration: 380; easing.type: Easing.OutCubic } }

    transform: Translate { id: nudge }

    Timer {
        id: failed
        interval: 900
        onTriggered: helix.failing = false
    }

    SequentialAnimation {
        id: shake
        NumberAnimation { target: nudge; property: "x"; to: 14 * helix.height / 140; duration: 45 }
        NumberAnimation { target: nudge; property: "x"; to: -10 * helix.height / 140; duration: 70 }
        NumberAnimation { target: nudge; property: "x"; to: 6 * helix.height / 140; duration: 60 }
        NumberAnimation { target: nudge; property: "x"; to: 0; duration: 50 }
    }

    FrameAnimation {
        running: helix.visible
        onTriggered: {
            helix.time += frameTime
            helix.phase = (helix.phase + (0.35 + helix.boost) * frameTime) % (2 * Math.PI)
            helix.boost *= Math.exp(-2.5 * frameTime)
        }
    }

    fragmentShader: "helix.frag.qsb"
}
