// Hand-written; qutegen never overwrites *_manual files.
#ifndef QUTE_GUI_VARIANT_MANUAL_H
#define QUTE_GUI_VARIANT_MANUAL_H
#include "qute_common.h"
#ifdef __cplusplus
extern "C" {
#endif
// The GUI types a QVariant can hold, by the numbers variant_manual.go uses.
enum {
    QUTE_GV_ICON = 1,
    QUTE_GV_FONT,
    QUTE_GV_BRUSH,
    QUTE_GV_PIXMAP,
    QUTE_GV_IMAGE,
    QUTE_GV_PEN,
    QUTE_GV_KEYSEQUENCE,
    QUTE_GV_CURSOR,
    QUTE_GV_PALETTE,
    QUTE_GV_TRANSFORM,
    QUTE_GV_REGION,
    QUTE_GV_COLOR = 100
};
void *qute_gui_variant_from(int kind, const void *value);
void *qute_gui_variant_color(qute_color c);
int qute_gui_variant_kind(const void *v);
void *qute_gui_variant_take(const void *v, int kind);
qute_color qute_gui_variant_to_color(const void *v);
void *qute_gui_image_from_bytes(const char *data, ptrdiff_t n, int width, int height, long long bytesPerLine, int64_t format);
#ifdef __cplusplus
}
#endif
#endif
