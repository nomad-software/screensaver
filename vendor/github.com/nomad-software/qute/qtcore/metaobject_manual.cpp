// Hand-written; qutegen never overwrites *_manual files.
#include "metaobject_manual.h"

#include <QtCore/QObject>

extern "C" {
const void *qute_qobject_meta_object(void *o) { return static_cast<QObject *>(o)->metaObject(); }
}
