// Hand-written; qutegen never overwrites *_manual files.
#ifndef QUTE_DYING_MANUAL_H
#define QUTE_DYING_MANUAL_H
#include <stdbool.h>
#ifdef __cplusplus
extern "C" {
#endif
bool qute_widget_in_destructor(void *obj);
#ifdef __cplusplus
}
#endif
#endif
