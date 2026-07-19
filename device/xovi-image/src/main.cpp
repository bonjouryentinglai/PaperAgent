// SPDX-License-Identifier: GPL-3.0-only
//
// Native Xochitl image insertion bridge for Paper Agent. The active
// DocumentView lookup is adapted from remarkable-doc-links' GPL-3.0
// remarkable-xovi-native plugin, pinned at
// 23e41858818ae2c1f836043852118a323cb1e77d.

#include <cstdlib>
#include <cstring>
#include <functional>
#include <cmath>

#include <QCoreApplication>
#include <QFile>
#include <QGuiApplication>
#include <QImage>
#include <QMetaMethod>
#include <QMetaType>
#include <QObject>
#include <QPointF>
#include <QQuickItem>
#include <QQuickWindow>
#include <QSet>
#include <QThread>
#include <QUrl>
#include <QVariant>
#include <QWindow>

namespace {

constexpr const char *kArtifactRoot = "/home/root/paper-agent/native/artifacts/";
constexpr const char *kResultRoot = "/run/paper-agent-image-";
QSet<QString> gCompletedRequests;

struct ImageRequest {
    QString id;
    QPointF position;
    bool valid = false;
};

bool isRequestId(const QString &value)
{
    if (value.size() < 10 || value.size() > 20)
        return false;
    for (const QChar ch : value) {
        if (!ch.isDigit())
            return false;
    }
    return true;
}

ImageRequest parseRequest(const QString &value)
{
    const QStringList fields = value.split(QLatin1Char(','));
    if (fields.size() != 3 || !isRequestId(fields[0]))
        return {};
    bool xOk = false;
    bool yOk = false;
    const double x = fields[1].toDouble(&xOk);
    const double y = fields[2].toDouble(&yOk);
    if (!xOk || !yOk || !std::isfinite(x) || !std::isfinite(y)
        || x < -8192.0 || x > 8192.0 || y < -8192.0 || y > 8192.0) {
        return {};
    }
    return { fields[0], QPointF(x, y), true };
}

void collectChildren(QObject *object, QSet<QObject *> &children)
{
    if (!object)
        return;
    if (auto *item = qobject_cast<QQuickItem *>(object)) {
        for (QQuickItem *child : item->childItems())
            children.insert(child);
    }
    for (QObject *child : object->children())
        children.insert(child);
}

QObject *findDocumentView(QObject *root, QSet<QObject *> &visited)
{
    if (!root || visited.contains(root))
        return nullptr;
    visited.insert(root);
    const QString className = QString::fromLatin1(root->metaObject()->className());
    if (className.contains(QStringLiteral("DocumentView"), Qt::CaseInsensitive)
        && root->property("documentLoaded").toBool()
        && root->property("notePage").toBool()) {
        return root;
    }
    QSet<QObject *> children;
    collectChildren(root, children);
    for (QObject *child : children) {
        if (QObject *found = findDocumentView(child, visited))
            return found;
    }
    return nullptr;
}

QObject *activeDocumentView()
{
    for (QWindow *window : QGuiApplication::allWindows()) {
        QSet<QObject *> visited;
        if (auto *quickWindow = qobject_cast<QQuickWindow *>(window)) {
            if (QObject *found = findDocumentView(quickWindow->contentItem(), visited))
                return found;
        }
        if (QObject *found = findDocumentView(window, visited))
            return found;
    }
    return nullptr;
}

QObject *objectProperty(QObject *object, const char *name)
{
    if (!object)
        return nullptr;
    const QVariant value = object->property(name);
    if (value.metaType().flags().testFlag(QMetaType::PointerToQObject))
        return qvariant_cast<QObject *>(value);
    return nullptr;
}

bool invokeImageFileInsertion(
    QObject *sceneController, const QString &imagePath, const QPointF &dropPosition)
{
    if (!sceneController || imagePath.isEmpty())
        return false;
    const QMetaObject *metaObject = sceneController->metaObject();
    for (int i = 0; i < metaObject->methodCount(); ++i) {
        const QMetaMethod method = metaObject->method(i);
        if (method.methodSignature()
            != QByteArrayLiteral("insertImageFileAsSceneItem(QUrl,QPointF)"))
            continue;
        QUrl imageUrl = QUrl::fromLocalFile(imagePath);
        QPointF position = dropPosition;
        return method.invoke(
            sceneController,
            Qt::DirectConnection,
            QGenericArgument("QUrl", &imageUrl),
            QGenericArgument("QPointF", &position)
        );
    }
    return false;
}

void writeResult(const QString &requestId, bool failed)
{
    QFile file(QString::fromUtf8(kResultRoot) + requestId
        + (failed ? QStringLiteral(".error") : QStringLiteral(".ack")));
    if (file.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        file.write(failed ? "error\n" : "ok\n");
        file.close();
    }
}

QString insertImage(const ImageRequest &request)
{
    if (!request.valid)
        return QStringLiteral("ERROR: invalid image request");
    const QString &requestId = request.id;
    if (gCompletedRequests.contains(requestId)) {
        writeResult(requestId, false);
        return QStringLiteral("ok: already inserted");
    }

    const QString imagePath = QString::fromUtf8(kArtifactRoot) + requestId
        + QStringLiteral(".png");
    QFile imageFile(imagePath);
    if (!imageFile.open(QIODevice::ReadOnly)) {
        writeResult(requestId, true);
        return QStringLiteral("ERROR: generated image is missing");
    }
    const QByteArray bytes = imageFile.readAll();
    imageFile.close();
    if (bytes.size() < 45 || bytes.size() > 32 * 1024 * 1024) {
        writeResult(requestId, true);
        return QStringLiteral("ERROR: generated image size is invalid");
    }
    QImage image;
    if (!image.loadFromData(bytes, "PNG") || image.width() < 32 || image.height() < 32
        || image.width() > 800 || image.height() > 800) {
        writeResult(requestId, true);
        return QStringLiteral("ERROR: generated PNG is invalid");
    }

    QObject *documentView = activeDocumentView();
    QObject *sceneController = objectProperty(documentView, "sceneController");
    if (!documentView || !sceneController) {
        writeResult(requestId, true);
        return QStringLiteral("ERROR: active notebook is unavailable");
    }

    if (!invokeImageFileInsertion(sceneController, imagePath, request.position)) {
        writeResult(requestId, true);
        return QStringLiteral("ERROR: Xochitl image insertion failed");
    }

    gCompletedRequests.insert(requestId);
    writeResult(requestId, false);
    return QStringLiteral("ok: native image inserted");
}

QString onGuiThread(const std::function<QString()> &callback)
{
    QCoreApplication *app = QCoreApplication::instance();
    if (!app)
        return QStringLiteral("ERROR: Xochitl application unavailable");
    if (QThread::currentThread() == app->thread())
        return callback();
    QString result = QStringLiteral("ERROR: GUI invocation failed");
    const bool invoked = QMetaObject::invokeMethod(
        app,
        [&result, &callback]() { result = callback(); },
        Qt::BlockingQueuedConnection
    );
    return invoked ? result : QStringLiteral("ERROR: GUI invocation failed");
}

char *copyResult(const QString &value)
{
    return ::strdup(value.toUtf8().constData());
}

} // namespace

extern "C" char *paperAgentImagePing(const char *value)
{
    return copyResult(QStringLiteral("pong:") + QString::fromUtf8(value ? value : ""));
}

extern "C" char *paperAgentInsertImage(const char *value)
{
    const ImageRequest request = parseRequest(QString::fromUtf8(value ? value : ""));
    return copyResult(onGuiThread([request]() { return insertImage(request); }));
}
