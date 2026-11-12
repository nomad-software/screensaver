// Hand-written; qutegen never overwrites *_manual files.
#include "variant_manual.h"

#include <QtCore/QStringList>
#include <QtCore/QVariant>

static QVariant &V(void *v) { return *static_cast<QVariant *>(v); }

extern "C" {
void *qute_variant_null(void) { return new QVariant(); }
void *qute_variant_bool(bool v) { return new QVariant(v); }
void *qute_variant_int(int v) { return new QVariant(v); }
void *qute_variant_int64(long long v) {
    if (v >= INT_MIN && v <= INT_MAX) return new QVariant(int(v));
    return new QVariant(qlonglong(v));
}
void *qute_variant_uint64(unsigned long long v) { return new QVariant(qulonglong(v)); }
void *qute_variant_double(double v) { return new QVariant(v); }
void *qute_variant_string(const char *d, ptrdiff_t n) { return new QVariant(QString::fromUtf8(d, n)); }
void *qute_variant_bytes(const char *d, ptrdiff_t n) { return new QVariant(QByteArray(d, n)); }
void *qute_variant_strlist(const char *d, const ptrdiff_t *lens, ptrdiff_t n) {
    return new QVariant(QStringList(qute_qstrlist(d, lens, n)));
}
void *qute_variant_point(qute_point p) { return new QVariant(qute_to_QPoint(p)); }
void *qute_variant_pointf(qute_pointf p) { return new QVariant(qute_to_QPointF(p)); }
void *qute_variant_size(qute_size s) { return new QVariant(qute_to_QSize(s)); }
void *qute_variant_rect(qute_rect r) { return new QVariant(qute_to_QRect(r)); }

int qute_variant_kind(void *v) {
    const QVariant &x = V(v);
    if (!x.isValid() || x.isNull()) return 0;
    switch (x.typeId()) {
    case QMetaType::Bool: return 1;
    case QMetaType::Int:
    case QMetaType::LongLong:
    case QMetaType::Short:
    case QMetaType::Long:
    case QMetaType::Char:
    case QMetaType::SChar: return 2;
    case QMetaType::UInt:
    case QMetaType::ULongLong:
    case QMetaType::UShort:
    case QMetaType::ULong:
    case QMetaType::UChar: return 3;
    case QMetaType::Double:
    case QMetaType::Float: return 4;
    case QMetaType::QString: return 5;
    case QMetaType::QByteArray: return 6;
    case QMetaType::QStringList: return 7;
    case QMetaType::QPoint: return 8;
    case QMetaType::QSize: return 9;
    case QMetaType::QRect: return 10;
    case QMetaType::QPointF: return 11;
    }
    if (x.canConvert<QString>()) return 12;
    return -1;
}
bool qute_variant_to_bool(void *v) { return V(v).toBool(); }
long long qute_variant_to_int64(void *v) { return V(v).toLongLong(); }
unsigned long long qute_variant_to_uint64(void *v) { return V(v).toULongLong(); }
double qute_variant_to_double(void *v) { return V(v).toDouble(); }
qute_str qute_variant_to_string(void *v) { return qute_ret_str(V(v).toString()); }
qute_str qute_variant_to_bytes(void *v) { return qute_ret_bytes(V(v).toByteArray()); }
qute_list qute_variant_to_strlist(void *v) { return qute_ret_strlist(V(v).toStringList()); }
qute_point qute_variant_to_point(void *v) { return qute_from_QPoint(V(v).toPoint()); }
qute_pointf qute_variant_to_pointf(void *v) { return qute_from_QPointF(V(v).toPointF()); }
qute_size qute_variant_to_size(void *v) { return qute_from_QSize(V(v).toSize()); }
qute_rect qute_variant_to_rect(void *v) { return qute_from_QRect(V(v).toRect()); }
}
