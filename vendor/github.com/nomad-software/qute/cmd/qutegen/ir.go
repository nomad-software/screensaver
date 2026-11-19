package main

import (
	"encoding/json"
	"os"
)

// The IR is written by tools/irgen/irgen.py from the Qt headers.

type irType struct {
	Spelling  string `json:"spelling"`
	Canonical string `json:"canonical"`
}

type irParam struct {
	Name    string `json:"name"`
	Type    irType `json:"type"`
	Default bool   `json:"default"`
}

type irMethod struct {
	Name       string    `json:"name"`
	Kind       string    `json:"kind"` // method, ctor, signal, slot
	Access     string    `json:"access"`
	Params     []irParam `json:"params"`
	Return     irType    `json:"return"`
	Static     bool      `json:"static"`
	Const      bool      `json:"const"`
	Virtual    bool      `json:"virtual"`
	Pure       bool      `json:"pure"`
	Final      bool      `json:"final"`
	Deprecated bool      `json:"deprecated"`
	Variadic   bool      `json:"variadic"`
	Copy       bool      `json:"copy"`
	Move       bool      `json:"move"`
	DefaultCtr bool      `json:"default_ctor"`
}

type irEnumValue struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

type irEnum struct {
	Name       string        `json:"name"`
	Scope      string        `json:"scope"`
	Scoped     bool          `json:"scoped"`
	Underlying string        `json:"underlying"`
	Values     []irEnumValue `json:"values"`
	// Protected enums type the protected methods a Go subclass calls; Go
	// binds them like public ones.
	Protected bool `json:"protected"`
}

type irFlags struct {
	Name string `json:"name"`
	Enum string `json:"enum"`
}

type irDtor struct {
	Access  string `json:"access"`
	Virtual bool   `json:"virtual"`
}

type irClass struct {
	Name     string     `json:"name"`
	Module   string     `json:"module"`
	Header   string     `json:"header"`
	Kind     string     `json:"kind"`
	Bases    []string   `json:"bases"`
	Abstract bool       `json:"abstract"`
	QObject  bool       `json:"qobject"`
	Gadget   bool       `json:"gadget"`
	Dtor     *irDtor    `json:"dtor"`
	Enums    []irEnum   `json:"enums"`
	Flags    []irFlags  `json:"flags"`
	Methods  []irMethod `json:"methods"`
	Fields   []irField  `json:"fields"`
	// Dropped lists what irgen leaves out of Methods (operators, conversion
	// operators, member templates); only the coverage report reads it.
	Dropped []irDropped `json:"dropped"`
}

type irDropped struct {
	Kind string `json:"kind"` // operator, conversion or template
	Name string `json:"name"`
}

type irField struct {
	Name string `json:"name"`
	Type irType `json:"type"`
}

type irModule struct {
	Module    string    `json:"module"`
	QtVersion string    `json:"qt_version"`
	Classes   []irClass `json:"classes"`
	Enums     []irEnum  `json:"enums"`
	Flags     []irFlags `json:"flags"`
	// GLConstants are the integer GL_* macros (QtGui only).
	GLConstants map[string]uint64 `json:"gl_constants"`
}

func loadIR(path string) (*irModule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m irModule
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
