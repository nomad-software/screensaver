// Hand-written; qutegen never overwrites *_manual files.
#include "variant_manual.h"

#include <QtCore/QVariant>
#include <QtGui/QBrush>
#include <QtGui/QColor>
#include <QtGui/QCursor>
#include <QtGui/QFont>
#include <QtGui/QIcon>
#include <QtGui/QImage>
#include <QtGui/QKeySequence>
#include <QtGui/QPalette>
#include <QtGui/QPen>
#include <QtGui/QPixmap>
#include <QtGui/QRegion>
#include <QtGui/QTransform>

#include <cstring>

namespace {
template <typename T> void *wrap(const void *p) { return new QVariant(QVariant::fromValue(*static_cast<const T *>(p))); }
template <typename T> void *take(const QVariant &v) { return new T(v.value<T>()); }
} // namespace

extern "C" {
void *qute_gui_variant_from(int kind, const void *p) {
    switch (kind) {
    case QUTE_GV_ICON: return wrap<QIcon>(p);
    case QUTE_GV_FONT: return wrap<QFont>(p);
    case QUTE_GV_BRUSH: return wrap<QBrush>(p);
    case QUTE_GV_PIXMAP: return wrap<QPixmap>(p);
    case QUTE_GV_IMAGE: return wrap<QImage>(p);
    case QUTE_GV_PEN: return wrap<QPen>(p);
    case QUTE_GV_KEYSEQUENCE: return wrap<QKeySequence>(p);
    case QUTE_GV_CURSOR: return wrap<QCursor>(p);
    case QUTE_GV_PALETTE: return wrap<QPalette>(p);
    case QUTE_GV_TRANSFORM: return wrap<QTransform>(p);
    case QUTE_GV_REGION: return wrap<QRegion>(p);
    }
    return new QVariant();
}

void *qute_gui_variant_color(qute_color c) { return new QVariant(qute_to_QColor(c)); }

int qute_gui_variant_kind(const void *v) {
    const QVariant &x = *static_cast<const QVariant *>(v);
    switch (x.typeId()) {
    case QMetaType::QIcon: return QUTE_GV_ICON;
    case QMetaType::QFont: return QUTE_GV_FONT;
    case QMetaType::QBrush: return QUTE_GV_BRUSH;
    case QMetaType::QPixmap: return QUTE_GV_PIXMAP;
    case QMetaType::QImage: return QUTE_GV_IMAGE;
    case QMetaType::QPen: return QUTE_GV_PEN;
    case QMetaType::QKeySequence: return QUTE_GV_KEYSEQUENCE;
    case QMetaType::QCursor: return QUTE_GV_CURSOR;
    case QMetaType::QPalette: return QUTE_GV_PALETTE;
    case QMetaType::QTransform: return QUTE_GV_TRANSFORM;
    case QMetaType::QRegion: return QUTE_GV_REGION;
    case QMetaType::QColor: return QUTE_GV_COLOR;
    }
    return 0;
}

void *qute_gui_variant_take(const void *v, int kind) {
    const QVariant &x = *static_cast<const QVariant *>(v);
    switch (kind) {
    case QUTE_GV_ICON: return take<QIcon>(x);
    case QUTE_GV_FONT: return take<QFont>(x);
    case QUTE_GV_BRUSH: return take<QBrush>(x);
    case QUTE_GV_PIXMAP: return take<QPixmap>(x);
    case QUTE_GV_IMAGE: return take<QImage>(x);
    case QUTE_GV_PEN: return take<QPen>(x);
    case QUTE_GV_KEYSEQUENCE: return take<QKeySequence>(x);
    case QUTE_GV_CURSOR: return take<QCursor>(x);
    case QUTE_GV_PALETTE: return take<QPalette>(x);
    case QUTE_GV_TRANSFORM: return take<QTransform>(x);
    case QUTE_GV_REGION: return take<QRegion>(x);
    }
    return nullptr;
}

qute_color qute_gui_variant_to_color(const void *v) {
    return qute_from_QColor(static_cast<const QVariant *>(v)->value<QColor>());
}

void *qute_gui_image_from_bytes(const char *data, ptrdiff_t n, int width, int height, long long bytesPerLine, int64_t format) {
    QImage img(width, height, QImage::Format(format));
    if (img.isNull()) return new QImage();
    img.fill(0);
    qsizetype rows = height, line = qsizetype(bytesPerLine), copy = qMin<qsizetype>(line, img.bytesPerLine());
    for (qsizetype y = 0; y < rows && (y + 1) * line <= n; y++) {
        std::memcpy(img.scanLine(int(y)), data + y * line, size_t(copy));
    }
    return new QImage(img);
}
}
