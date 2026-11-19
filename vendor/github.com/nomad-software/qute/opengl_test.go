package qute_test

import (
	"testing"
	"unsafe"

	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtopengl"
	"github.com/nomad-software/qute/qtopenglwidgets"
	"github.com/nomad-software/qute/qutetest"
)

// glTriangle draws one red triangle that covers the centre of the widget,
// through a shader program, a vertex buffer and a vertex array object.
type glTriangle struct {
	qtopenglwidgets.QOpenGLWidgetBase
	prog *qtopengl.QOpenGLShaderProgram
	vbo  *qtopengl.QOpenGLBuffer
	vao  *qtopengl.QOpenGLVertexArrayObject
	err  string
}

const vertexSrc = `
attribute vec2 pos;
void main() { gl_Position = vec4(pos, 0.0, 1.0); }`

const fragmentSrc = `
uniform vec4 colour;
void main() { gl_FragColor = colour; }`

func (g *glTriangle) InitializeGL() {
	g.prog = qtopengl.NewQOpenGLShaderProgram(g.QOpenGLWidget)
	if !g.prog.AddShaderFromSourceCode(qtopengl.QOpenGLShaderVertex, vertexSrc) ||
		!g.prog.AddShaderFromSourceCode(qtopengl.QOpenGLShaderFragment, fragmentSrc) || !g.prog.Link() {
		g.err = g.prog.Log()
		return
	}
	g.vao = qtopengl.NewQOpenGLVertexArrayObject(g.QOpenGLWidget)
	g.vao.Create()
	g.vao.Bind()
	verts := []float32{-1, -1, 1, -1, 0, 1}
	g.vbo = qtopengl.NewQOpenGLBufferWithType(qtopengl.QOpenGLBufferVertexBuffer)
	g.vbo.Create()
	g.vbo.Bind()
	g.vbo.AllocatePtr(unsafe.Pointer(&verts[0]), len(verts)*4) // copied into GL memory
	g.prog.Bind()
	loc := g.prog.AttributeLocation("pos")
	g.prog.EnableAttributeArray(loc)
	g.prog.SetAttributeBuffer(loc, qtgui.GL_FLOAT, 0, 2)
	g.vao.Release()
}

func (g *glTriangle) PaintGL() {
	if g.err != "" {
		return
	}
	gl := g.Context().Functions()
	gl.GlClearColor(0, 0, 0, 1)
	gl.GlClear(qtgui.GL_COLOR_BUFFER_BIT)
	g.prog.Bind()
	g.prog.SetUniformVector4D(g.prog.UniformLocation("colour"), qtgui.NewQVector4DWithXpos(1, 0, 0, 1))
	g.vao.Bind()
	gl.GlDrawArrays(qtgui.GL_TRIANGLES, 0, 3)
	g.vao.Release()
}

func TestOpenGLTriangle(t *testing.T) {
	qutetest.Run(t, func() {
		g := &glTriangle{}
		w := qtopenglwidgets.NewQOpenGLWidgetWith(g, nil)
		defer w.Delete()
		w.Resize(64, 64)
		img := w.GrabFramebuffer()
		if g.prog == nil {
			t.Skip("no OpenGL context available on this platform")
		}
		if g.err != "" {
			t.Fatalf("shader build failed: %s", g.err)
		}
		if c := img.PixelColor(32, 32); c.R != 255 || c.G != 0 || c.B != 0 {
			t.Errorf("centre pixel = %+v, want red", c)
		}
		if c := img.PixelColor(2, 62); c.R != 0 && c.G != 0 {
			t.Errorf("corner pixel = %+v, want the black background", c)
		}
	})
}
