// Hand-written; qutegen never overwrites *_manual.go files.

// Package qtopenglwidgets binds QOpenGLWidget. Subclass it from Go by
// embedding QOpenGLWidgetBase and defining InitializeGL, ResizeGL and PaintGL:
//
//	type Canvas struct{ qtopenglwidgets.QOpenGLWidgetBase }
//
//	func (c *Canvas) PaintGL() {
//		gl := c.Context().Functions()
//		gl.GlClearColor(0, 0, 0, 1)
//		gl.GlClear(qtgui.GL_COLOR_BUFFER_BIT)
//	}
//
//	qtopenglwidgets.NewQOpenGLWidgetWith(&Canvas{}, nil)
//
// GrabFramebuffer renders the widget into a QImage, which works headlessly
// under QT_QPA_PLATFORM=offscreen when the system has an OpenGL driver.
package qtopenglwidgets
