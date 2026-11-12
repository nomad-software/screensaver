#ifndef QUTE_TOOLS_H
#define QUTE_TOOLS_H
#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#ifdef __cplusplus
extern "C" {
#endif
typedef struct { unsigned char *data; int w, h; } qute_rgba;
typedef struct {
    char *cls;
    char *name;
    bool widget;
    bool visible;
    bool window;
    int x, y, w, h;
    void **children;
    int nchildren;
} qute_objinfo;

qute_rgba qute_tools_grab(void *widget);
void qute_tools_info(void *obj, qute_objinfo *out);
void qute_tools_free_info(qute_objinfo *info);
void qute_tools_process(void);
#ifdef __cplusplus
}
#endif
#endif
