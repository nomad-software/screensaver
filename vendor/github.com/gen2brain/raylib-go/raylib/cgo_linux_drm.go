//go:build linux && drm && !rpi && !android
// +build linux,drm,!rpi,!android

package rl

/*
#cgo linux,drm LDFLAGS: -lGLESv2 -lEGL -ldrm -lgbm -lpthread -lrt -lm -ldl
#cgo linux,drm CFLAGS: -DPLATFORM_DRM -DGRAPHICS_API_OPENGL_ES2 -DEGL_NO_X11 -I/usr/include/libdrm
*/
import "C"
