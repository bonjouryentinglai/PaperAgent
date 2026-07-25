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

    function acceptState(contents, applied) {
        var data
        try {
            data = JSON.parse(contents)
        } catch (error) {
            statusText = "Settings backend returned invalid data."
            applying = false
            return
        }
        serviceActive = data.service && data.service.active === true
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
        statusText = serviceActive
            ? "Paper Agent is busy. Apply will be available when it finishes."
            : (applied ? "Settings applied. Paper Agent is ready." : "Paper Agent is ready.")
        if (applied && exitAfterApply) {
            exitAfterApply = false
            root.close()
        }
    }

    function applyChanges(andExit) {
        if (!dirty || applying || serviceActive)
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
                root.acceptState(contents, true)
            } else if (type === 199) {
                root.applying = false
                root.exitAfterApply = false
                root.statusText = contents
            }
        }
    }

    Timer {
        interval: 2500
        repeat: true
        running: root.loaded && !root.applying && !root.confirmExit
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

        ActionButton {
            id: exitButton
            anchors.left: parent.left
            anchors.leftMargin: 28
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(220, parent.width * 0.24)
            height: 88
            label: "Exit"
            enabled: !root.applying
            onClicked: {
                if (root.dirty)
                    root.confirmExit = true
                else
                    root.close()
            }
        }

        Text {
            anchors.centerIn: parent
            text: "Paper Agent Settings"
            font.pixelSize: Math.max(34, Math.min(50, parent.height * 0.34))
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

        Text {
            width: parent.width
            text: root.statusText
            wrapMode: Text.WordWrap
            horizontalAlignment: Text.AlignHCenter
            font.pixelSize: 25
            color: root.serviceActive ? "#6b4c00" : "#3d403b"
        }

        ActionButton {
            width: parent.width
            height: 104
            label: root.applying ? "Applying…" : "Apply"
            enabled: root.loaded && root.dirty && !root.applying && !root.serviceActive
            onClicked: root.applyChanges(false)
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
                enabled: !root.serviceActive
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
