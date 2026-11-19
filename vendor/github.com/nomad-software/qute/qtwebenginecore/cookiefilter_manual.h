// Hand-written; qutegen never overwrites *_manual files.
#ifndef QUTE_COOKIEFILTER_MANUAL_H
#define QUTE_COOKIEFILTER_MANUAL_H
#include <stdbool.h>
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
// Install (id != 0) or remove (id == 0) the cookie filter of store. Qt calls
// the filter on a network thread, so it gets plain values, never Qt objects.
void qute_webengine_set_cookie_filter(void *store, uintptr_t id);
#ifdef __cplusplus
}
#endif
#endif
