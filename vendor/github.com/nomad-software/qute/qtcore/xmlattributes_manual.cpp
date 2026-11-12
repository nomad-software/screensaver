// Hand-written; qutegen never overwrites *_manual files.
#include "xmlattributes_manual.h"

#include <QtCore/QXmlStreamAttributes>

extern "C" {
ptrdiff_t qute_xmlattributes_size(void *a) { return static_cast<QXmlStreamAttributes *>(a)->size(); }

void *qute_xmlattributes_at(void *a, ptrdiff_t i) {
	return new QXmlStreamAttribute(static_cast<QXmlStreamAttributes *>(a)->at(i));
}
}
