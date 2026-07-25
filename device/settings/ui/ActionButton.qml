// SPDX-License-Identifier: MIT
import QtQuick 2.5

Rectangle {
    id: root
    property string label: ""
    signal clicked

    radius: 12
    border.width: 3
    border.color: enabled ? "#20242a" : "#9a9a96"
    color: enabled ? "#f8f7f2" : "#e7e5df"

    Text {
        anchors.centerIn: parent
        text: root.label
        color: root.enabled ? "#20242a" : "#777772"
        font.pixelSize: Math.max(24, Math.min(38, root.height * 0.32))
        font.bold: true
    }

    MouseArea {
        anchors.fill: parent
        enabled: root.enabled
        onClicked: root.clicked()
    }
}
