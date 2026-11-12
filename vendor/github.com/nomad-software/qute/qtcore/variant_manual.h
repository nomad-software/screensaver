// Hand-written; qutegen never overwrites *_manual files.
#ifndef QUTE_VARIANT_MANUAL_H
#define QUTE_VARIANT_MANUAL_H
#include "qute_common.h"
#ifdef __cplusplus
extern "C" {
#endif
void *qute_variant_null(void);
void *qute_variant_bool(bool v);
void *qute_variant_int(int v);
void *qute_variant_int64(long long v);
void *qute_variant_uint64(unsigned long long v);
void *qute_variant_double(double v);
void *qute_variant_string(const char *d, ptrdiff_t n);
void *qute_variant_bytes(const char *d, ptrdiff_t n);
void *qute_variant_strlist(const char *d, const ptrdiff_t *lens, ptrdiff_t n);
void *qute_variant_point(qute_point p);
void *qute_variant_pointf(qute_pointf p);
void *qute_variant_size(qute_size s);
void *qute_variant_rect(qute_rect r);
int qute_variant_kind(void *v);
bool qute_variant_to_bool(void *v);
long long qute_variant_to_int64(void *v);
unsigned long long qute_variant_to_uint64(void *v);
double qute_variant_to_double(void *v);
qute_str qute_variant_to_string(void *v);
qute_str qute_variant_to_bytes(void *v);
qute_list qute_variant_to_strlist(void *v);
qute_point qute_variant_to_point(void *v);
qute_pointf qute_variant_to_pointf(void *v);
qute_size qute_variant_to_size(void *v);
qute_rect qute_variant_to_rect(void *v);
#ifdef __cplusplus
}
#endif
#endif
