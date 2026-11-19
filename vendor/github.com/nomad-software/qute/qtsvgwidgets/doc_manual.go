// Hand-written; qutegen never overwrites *_manual.go files.

// Package qtsvgwidgets binds QtSvgWidgets: QSvgWidget shows an SVG file in a
// widget, and QGraphicsSvgItem shows one in a QGraphicsScene. It is generated
// by cmd/qutegen from spec/.
//
// An item given a shared renderer (SetSharedRenderer) keeps it alive, so one
// renderer can serve many items.
package qtsvgwidgets
