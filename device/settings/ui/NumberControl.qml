// SPDX-License-Identifier: MIT
import QtQuick 2.5

Rectangle {
    id: root
    property int value: 100
    property int minimum: 0
    property int maximum: 100
    property int step: 5
    property string suffix: "%"
    signal valueSelected(int value)

    radius: 10
    border.width: 2
    border.color: "#20242a"
    color: "#fbfaf6"

    function adjust(delta) {
        var candidate = Math.max(minimum, Math.min(maximum, value + delta))
        valueSelected(candidate)
    }

    Rectangle {
        id: minus
        width: Math.min(110, parent.width * 0.24)
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        color: "transparent"
        Text {
            anchors.centerIn: parent
            text: "−"
            font.pixelSize: 48
            color: root.value > root.minimum ? "#20242a" : "#9a9a96"
        }
        MouseArea {
            anchors.fill: parent
            enabled: root.value > root.minimum
            onClicked: root.adjust(-root.step)
        }
    }

    Text {
        anchors.left: minus.right
        anchors.right: plus.left
        anchors.verticalCenter: parent.verticalCenter
        text: root.value + root.suffix
        horizontalAlignment: Text.AlignHCenter
        font.pixelSize: Math.max(26, Math.min(38, root.height * 0.32))
        color: "#20242a"
    }

    Rectangle {
        id: plus
        width: minus.width
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        color: "transparent"
        Text {
            anchors.centerIn: parent
            text: "+"
            font.pixelSize: 48
            color: root.value < root.maximum ? "#20242a" : "#9a9a96"
        }
        MouseArea {
            anchors.fill: parent
            enabled: root.value < root.maximum
            onClicked: root.adjust(root.step)
        }
    }
}
