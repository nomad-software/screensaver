#include "tools.h"

#include <QtCore/QCoreApplication>
#include <QtCore/QEventLoop>
#include <QtGui/QImage>
#include <QtGui/QPixmap>
#include <QtWidgets/QLayout>
#include <QtWidgets/QWidget>

#include <cstring>

static char *dup(const QByteArray &b) {
    char *p = static_cast<char *>(std::malloc(b.size() + 1));
    std::memcpy(p, b.constData(), b.size());
    p[b.size()] = 0;
    return p;
}

extern "C" qute_rgba qute_tools_grab(void *widget) {
    QWidget *w = static_cast<QWidget *>(widget);
    // A widget that was never shown has not been laid out yet.
    w->ensurePolished();
    if (!w->isVisible()) {
        if (QLayout *l = w->layout()) l->activate();
        if (!w->testAttribute(Qt::WA_Resized)) w->adjustSize();
    }
    QImage img = w->grab().toImage().convertToFormat(QImage::Format_RGBA8888);
    qute_rgba out{nullptr, img.width(), img.height()};
    const int row = img.width() * 4;
    out.data = static_cast<unsigned char *>(std::malloc(size_t(row) * img.height() + 1));
    for (int y = 0; y < img.height(); y++) {
        std::memcpy(out.data + size_t(y) * row, img.constScanLine(y), row);
    }
    return out;
}

extern "C" void qute_tools_info(void *obj, qute_objinfo *out) {
    QObject *o = static_cast<QObject *>(obj);
    out->cls = dup(QByteArray(o->metaObject()->className()));
    out->name = dup(o->objectName().toUtf8());
    out->widget = o->isWidgetType();
    out->visible = false;
    out->window = false;
    out->x = out->y = out->w = out->h = 0;
    if (out->widget) {
        QWidget *w = static_cast<QWidget *>(o);
        out->visible = w->isVisible();
        out->window = w->isWindow();
        QRect g = w->geometry();
        out->x = g.x();
        out->y = g.y();
        out->w = g.width();
        out->h = g.height();
    }
    const QObjectList &ch = o->children();
    out->nchildren = int(ch.size());
    out->children = static_cast<void **>(std::malloc(sizeof(void *) * (ch.size() + 1)));
    for (int i = 0; i < ch.size(); i++) out->children[i] = ch[i];
}

extern "C" void qute_tools_free_info(qute_objinfo *info) {
    std::free(info->cls);
    std::free(info->name);
    std::free(info->children);
}

extern "C" void qute_tools_process(void) { QCoreApplication::processEvents(QEventLoop::AllEvents); }
