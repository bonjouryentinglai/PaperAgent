// SPDX-License-Identifier: MIT
import QtQuick 2.5
import net.asivery.AppLoad 1.0
import net.asivery.ApploadUtils

Rectangle {
    id: root
    anchors.fill: parent
    color: "#f5f3ed"

    signal close

    property bool loaded: false
    property bool applying: false
    property bool restarting: false
    property bool serviceReady: false
    property bool serviceActive: false
    property bool exitAfterApply: false
    property bool confirmExit: false
    property string statusText: "Loading Paper Agent settings…"

    property var modelOptions: []
    property var thinkingOptions: []
    property string savedModel: ""
    property string savedThinking: ""
    property int savedTextScale: 100
    property int savedMinScale: 60
    property string editModel: ""
    property string editThinking: ""
    property int editTextScale: 100
    property int editMinScale: 60
    property bool dirty: loaded && (
        editModel !== savedModel
        || editThinking !== savedThinking
        || editTextScale !== savedTextScale
        || editMinScale !== savedMinScale
    )

    function unloading() {
        endpoint.terminate()
    }

    function settingsPayload() {
        return {
            model: editModel,
            thinking: editThinking,
            textScalePercent: editTextScale,
            minAutoScalePercent: editMinScale
        }
    }

    function serviceLabel() {
        if (restarting)
            return "Restarting"
        if (!serviceReady)
            return "Unavailable"
        return serviceActive ? "Busy" : "Ready"
    }

    function acceptState(contents, applied, message) {
        var data
        try {
            data = JSON.parse(contents)
        } catch (error) {
            statusText = "Settings backend returned invalid data."
            applying = false
            return
        }
        serviceReady = data.service && data.service.ready === true
        serviceActive = serviceReady && data.service.active === true
        modelOptions = data.modelOptions || []
        thinkingOptions = data.thinkingOptions || []
        if (!loaded || !dirty || applied) {
            savedModel = data.settings.model
            savedThinking = data.settings.thinking
            savedTextScale = data.settings.textScalePercent
            savedMinScale = data.settings.minAutoScalePercent
            editModel = savedModel
            editThinking = savedThinking
            editTextScale = savedTextScale
            editMinScale = savedMinScale
        }
        loaded = true
        applying = false
        restarting = false
        statusText = message || (serviceActive
            ? "Paper Agent is busy. Apply will be available when it finishes."
            : (serviceReady
                ? (applied ? "Settings applied. Paper Agent is ready." : "Paper Agent is ready.")
                : "Paper Agent service is unavailable. Use Restart to recover it."))
        if (applied && exitAfterApply) {
            exitAfterApply = false
            root.close()
        }
    }

    function applyChanges(andExit) {
        if (!dirty || applying || restarting || serviceActive)
            return
        exitAfterApply = andExit
        confirmExit = false
        applying = true
        statusText = "Applying settings and restarting Paper Agent…"
        endpoint.sendMessage(2, JSON.stringify(settingsPayload()))
    }

    AppLoad {
        id: endpoint
        applicationID: "paper-agent-settings"
        onMessageReceived: (type, contents) => {
            if (type === 100) {
                root.acceptState(contents, false)
            } else if (type === 101) {
                root.applying = true
                root.statusText = "Applying settings and restarting Paper Agent…"
            } else if (type === 102) {
                root.acceptState(contents, true, "")
            } else if (type === 103) {
                root.restarting = true
                root.statusText = "Restarting Paper Agent service…"
            } else if (type === 104) {
                root.acceptState(contents, false, "Paper Agent restarted and is ready.")
            } else if (type === 199) {
                root.applying = false
                root.restarting = false
                root.exitAfterApply = false
                root.statusText = contents
            }
        }
    }

    Timer {
        interval: 2500
        repeat: true
        running: root.loaded && !root.applying && !root.restarting && !root.confirmExit
        onTriggered: endpoint.sendMessage(1, "")
    }

    DisplayMethodArea {
        anchors.fill: parent
        displayMethod: DisplayMethodArea.UI
    }

    Rectangle {
        id: header
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: Math.max(130, parent.height * 0.09)
        color: "#f5f3ed"
        property real sideSlotWidth: Math.min(190, width * 0.20)

        ActionButton {
            id: exitButton
            anchors.left: parent.left
            anchors.leftMargin: 24
            anchors.verticalCenter: parent.verticalCenter
            width: header.sideSlotWidth - 48
            height: 80
            label: "Exit"
            enabled: !root.applying && !root.restarting
            onClicked: {
                if (root.dirty)
                    root.confirmExit = true
                else
                    root.close()
            }
        }

        Text {
            anchors.left: parent.left
            anchors.leftMargin: header.sideSlotWidth
            anchors.right: parent.right
            anchors.rightMargin: header.sideSlotWidth
            anchors.verticalCenter: parent.verticalCenter
            text: "Paper Agent Settings"
            horizontalAlignment: Text.AlignHCenter
            elide: Text.ElideRight
            font.pixelSize: Math.max(32, Math.min(44, parent.height * 0.32))
            font.bold: true
            color: "#20242a"
        }

        Rectangle {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: 3
            color: "#20242a"
        }
    }

    Column {
        id: content
        anchors.top: header.bottom
        anchors.topMargin: 42
        anchors.left: parent.left
        anchors.leftMargin: Math.max(42, parent.width * 0.08)
        anchors.right: parent.right
        anchors.rightMargin: Math.max(42, parent.width * 0.08)
        spacing: 28
        opacity: root.loaded ? 1 : 0.45

        Text {
            width: parent.width
            text: "Model"
            font.pixelSize: 32
            font.bold: true
            color: "#20242a"
        }
        CycleControl {
            width: parent.width
            height: 112
            values: root.modelOptions
            value: root.editModel
            onValueSelected: root.editModel = value
        }

        Text {
            width: parent.width
            text: "Thinking"
            font.pixelSize: 32
            font.bold: true
            color: "#20242a"
        }
        CycleControl {
            width: parent.width
            height: 112
            values: root.thinkingOptions
            value: root.editThinking
            onValueSelected: root.editThinking = value
        }

        Text {
            width: parent.width
            text: "AI answer text size"
            font.pixelSize: 32
            font.bold: true
            color: "#20242a"
        }
        NumberControl {
            width: parent.width
            height: 112
            value: root.editTextScale
            minimum: 70
            maximum: 160
            step: 5
            onValueSelected: root.editTextScale = value
        }

        Text {
            width: parent.width
            text: "Smallest automatic text size"
            font.pixelSize: 32
            font.bold: true
            color: "#20242a"
        }
        NumberControl {
            width: parent.width
            height: 112
            value: root.editMinScale
            minimum: 40
            maximum: 100
            step: 5
            onValueSelected: root.editMinScale = value
        }
        Text {
            width: parent.width
            text: "Relative to the AI answer size. If the result still does not fit, Paper Agent starts a new page."
            wrapMode: Text.WordWrap
            font.pixelSize: 25
            color: "#555650"
        }
    }

    Column {
        anchors.left: content.left
        anchors.right: content.right
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 36
        spacing: 20

        Rectangle {
            width: parent.width
            height: 76
            radius: 12
            border.width: 3
            border.color: root.serviceReady ? (root.serviceActive ? "#8a6818" : "#315f3b") : "#8b3434"
            color: "#eeebe3"

            Row {
                anchors.centerIn: parent
                spacing: 16

                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    width: 22
                    height: 22
                    radius: 11
                    color: root.restarting ? "#8a6818"
                        : (root.serviceReady ? (root.serviceActive ? "#8a6818" : "#315f3b") : "#8b3434")
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "Service status: " + root.serviceLabel()
                    font.pixelSize: 28
                    font.bold: true
                    color: "#20242a"
                }
            }
        }

        Text {
            width: parent.width
            text: root.statusText
            wrapMode: Text.WordWrap
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 25
            color: root.serviceActive ? "#6b4c00" : (root.serviceReady ? "#3d403b" : "#7a2f2f")
        }

        Row {
            width: parent.width
            height: 104
            spacing: 18

            ActionButton {
                width: (parent.width - parent.spacing) / 2
                height: parent.height
                label: root.restarting ? "Restarting…" : "Restart service"
                enabled: root.loaded && !root.applying && !root.restarting
                onClicked: {
                    root.restarting = true
                    root.statusText = "Restarting Paper Agent service…"
                    endpoint.sendMessage(3, "")
                }
            }

            ActionButton {
                width: (parent.width - parent.spacing) / 2
                height: parent.height
                label: root.applying ? "Applying…" : "Apply"
                enabled: root.loaded && root.dirty && !root.applying && !root.restarting && !root.serviceActive
                onClicked: root.applyChanges(false)
            }
        }
    }

    Rectangle {
        anchors.fill: parent
        visible: root.confirmExit
        color: "#d8d6cf"
        opacity: 0.98
        z: 20

        Column {
            anchors.centerIn: parent
            width: parent.width * 0.78
            spacing: 28

            Text {
                width: parent.width
                text: "Unsaved changes"
                horizontalAlignment: Text.AlignHCenter
                font.pixelSize: 44
                font.bold: true
                color: "#20242a"
            }
            Text {
                width: parent.width
                text: "Apply your changes before leaving?"
                horizontalAlignment: Text.AlignHCenter
                wrapMode: Text.WordWrap
                font.pixelSize: 30
                color: "#20242a"
            }
            ActionButton {
                width: parent.width
                height: 104
                label: "Apply & Exit"
                enabled: !root.serviceActive && !root.restarting
                onClicked: root.applyChanges(true)
            }
            ActionButton {
                width: parent.width
                height: 104
                label: "Discard"
                onClicked: root.close()
            }
            ActionButton {
                width: parent.width
                height: 104
                label: "Cancel"
                onClicked: root.confirmExit = false
            }
        }
    }
}
