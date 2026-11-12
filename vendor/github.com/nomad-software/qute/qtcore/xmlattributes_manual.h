// Hand-written; qutegen never overwrites *_manual files.
#ifndef QUTE_XMLATTRIBUTES_MANUAL_H
#define QUTE_XMLATTRIBUTES_MANUAL_H
#include "qute_common.h"
#ifdef __cplusplus
extern "C" {
#endif
ptrdiff_t qute_xmlattributes_size(void *a);
void *qute_xmlattributes_at(void *a, ptrdiff_t i);
#ifdef __cplusplus
}
#endif
#endif
