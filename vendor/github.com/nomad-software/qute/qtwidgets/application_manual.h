// Hand-written; qutegen never overwrites *_manual files.
#ifndef QUTE_APPLICATION_MANUAL_H
#define QUTE_APPLICATION_MANUAL_H
#include "qute_common.h"
#ifdef __cplusplus
extern "C" {
#endif
void *qute_application_new(const char *args, const ptrdiff_t *lens, ptrdiff_t n);
#ifdef __cplusplus
}
#endif
#endif
