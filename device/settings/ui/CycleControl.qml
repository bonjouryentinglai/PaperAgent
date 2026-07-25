// SPDX-License-Identifier: MIT
import QtQuick 2.5

Rectangle {
    id: root
    property var values: []
    property string value: ""
    signal valueSelected(string value)

    radius: 10
    border.width: 2
    border.color: "#20242a"
    color: "#fbfaf6"

    function move(delta) {
        if (!values || values.length === 0)
            return
        var index = values.indexOf(value)
        if (index < 0)
            index = 0
        index = (index + delta + values.length) % values.length
        valueSelected(values[index])
    }

    Rectangle {
        id: previous
        width: Math.min(100, parent.width * 0.22)
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        color: "transparent"
        Text {
            anchors.centerIn: parent
            text: "‹"
            font.pixelSize: 56
            color: "#20242a"
        }
        MouseArea {
            anchors.fill: parent
            onClicked: root.move(-1)
        }
    }

    Text {
        anchors.left: previous.right
        anchors.right: next.left
        anchors.verticalCenter: parent.verticalCenter
        text: root.value
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideMiddle
        font.pixelSize: Math.max(24, Math.min(36, root.height * 0.30))
        color: "#20242a"
    }

    Rectangle {
        id: next
        width: previous.width
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        color: "transparent"
        Text {
            anchors.centerIn: parent
            text: "›"
            font.pixelSize: 56
            color: "#20242a"
        }
        MouseArea {
            anchors.fill: parent
            onClicked: root.move(1)
        }
    }
}
