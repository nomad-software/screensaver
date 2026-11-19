package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Class options live in spec/classes/<module>.yaml; ownership overlays live in
// spec/overlays/<module>.yaml. Overlays have the shape
//
//	QLayout:
//	  addWidget:  { args: { w: transfer-to-cpp } }
//	  takeAt:     { return: transfer-to-go }
//
// A method key is either a bare name (all overloads) or a signature such as
// "QPushButton(const QString&, QWidget*)" or "resize(const QSize&)".

type classOpts struct {
	// Kind: object (default for QObjects and polymorphic classes), value
	// (copyable Qt value class held in an owned handle) or event.
	Kind string `yaml:"kind"`
	// SubclassOpt is the spec's subclass option. Unset, every class that can
	// be subclassed from Go is (a constructible QObject or polymorphic class
	// with virtuals Go can implement); false turns it off, and true insists on
	// it, so the generator warns when the class cannot be.
	SubclassOpt *bool `yaml:"subclass"`
	// Subclass says the class gets a shim with Go-overridable virtuals and a
	// Base type; the generator decides it from SubclassOpt.
	Subclass bool `yaml:"-"`
	// NoSubclass is why a class Go cannot subclass is left out, for COVERAGE.md.
	NoSubclass string `yaml:"-"`
	// NotifyDtor generates a shim whose destructor tells Go the object died
	// (for non-QObject classes with a virtual destructor).
	NotifyDtor bool `yaml:"notify-dtor"`
	// NoConstruct hides constructors.
	NoConstruct bool `yaml:"no-construct"`
	// SkipVirtuals lists virtuals that may not be overridden from Go.
	SkipVirtuals []string `yaml:"skip-virtuals"`
	// Reviewed marks every default rule on the class as reviewed.
	Reviewed bool `yaml:"reviewed"`
	// RawPointers passes every pointer to a number as unsafe.Pointer instead of
	// an out-parameter or slice (OpenGL function tables, where GL decides the size).
	RawPointers bool `yaml:"raw-pointers"`
	// Inline keeps a value's bytes in Go memory instead of a C++ heap copy
	// (rt/inline.go). Only for a class that is trivially copyable and
	// destructible, which the shim checks with a static_assert.
	Inline bool `yaml:"inline"`
	// EnumsOnly emits the class's enums but no wrapper (QChar is a rune in Go).
	EnumsOnly bool `yaml:"enums-only"`
	// DependsOn lists the objects this value is only valid with. Each entry is
	// a const getter returning an object pointer (QModelIndex: model), or
	// {cpp: <C++ expression over $self>, class: <bound class>} when no getter
	// exists (a QTextTableCell's table). Every wrapper holds a reference to
	// those objects, so GC cannot free them first, and panics once one of them
	// is destroyed. A single getter may be written as a plain string.
	DependsOn depList `yaml:"depends-on"`
	// Memory says how many bytes one object holds outside the Go heap (a
	// QImage's pixels), so GC runs as those add up: size is a C++ expression
	// over $self, changed-by lists the methods after which it is measured again.
	Memory *memSpec `yaml:"memory"`
	// ParentedDelete makes Delete() of a Parented object of this class (and
	// of its non-QObject subclasses) panic with this fix: its container keeps
	// a pointer to it and does not notice its destructor (QLayoutItem).
	ParentedDelete string `yaml:"parented-delete"`
	// Cast, on the root of a value hierarchy, names the C++ cast template
	// that finds a derived object behind a pointer to the root (QStyleOption:
	// qstyleoption_cast, which reads the option's type and version). Every
	// class below the root gets <Class>Cast(o <Root>Like), whose result
	// depends on o.
	Cast string `yaml:"cast"`
}

type memSpec struct {
	Size      string   `yaml:"size"`
	ChangedBy []string `yaml:"changed-by"`
}

// depSpec is one depends-on entry: Getter, or Cpp with the Class it returns.
type depSpec struct {
	Getter string
	Cpp    string `yaml:"cpp"`
	Class  string `yaml:"class"`
}

type depList []depSpec

func (d *depList) UnmarshalYAML(n *yaml.Node) error {
	one := func(n *yaml.Node) (depSpec, error) {
		var s depSpec
		if n.Kind == yaml.ScalarNode {
			s.Getter = n.Value
			return s, nil
		}
		var m struct {
			Cpp   string `yaml:"cpp"`
			Class string `yaml:"class"`
		}
		if err := n.Decode(&m); err != nil {
			return s, err
		}
		if m.Cpp == "" || m.Class == "" {
			return s, fmt.Errorf("line %d: a depends-on entry needs both cpp and class", n.Line)
		}
		s.Cpp, s.Class = m.Cpp, m.Class
		return s, nil
	}
	if n.Kind != yaml.SequenceNode {
		s, err := one(n)
		*d = depList{s}
		return err
	}
	for _, e := range n.Content {
		s, err := one(e)
		if err != nil {
			return err
		}
		*d = append(*d, s)
	}
	return nil
}

type classSpec struct {
	Module  string                `yaml:"module"`
	Package string                `yaml:"package"`
	Classes map[string]*classOpts `yaml:"classes"`
}

type overlay struct {
	Args     map[string]string `yaml:"args"`
	Return   string            `yaml:"return"`
	Name     string            `yaml:"name"`
	Full     string            `yaml:"full"`
	KeepDefs bool              `yaml:"keep-defaults"`
	// NoFull says why the method has no Full form, where one cannot or should
	// not be built (a defaulted argument that is an input array).
	NoFull   string `yaml:"no-full"`
	Skip     any    `yaml:"skip"`
	Error    bool   `yaml:"error"`
	ErrorMsg string `yaml:"error-string"`
	Cpp      string `yaml:"cpp"`     // C++ call expression: $self, $0, $1 ... for the arguments
	Created  bool   `yaml:"created"` // the result is built by the binding, so its death is tracked
	Reviewed bool   `yaml:"reviewed"`
	Test     string `yaml:"test"`
	Doc      string `yaml:"doc"`
	// ReleaseSlot names a keep slot the method empties: after it returns, Qt
	// no longer uses what the slot held (QPainter::end and its device).
	ReleaseSlot string `yaml:"release-slot"`
	// KeptDelete says whether Delete() of what the method's keep rules keep
	// is allowed while the receiver lives: "allow" when Qt notices the kept
	// object's destruction, "guard" when it would follow a dangling pointer.
	// The default guards when the receiver or the kept argument is not a
	// QObject, since only QObject-to-QObject references are usually watched.
	KeptDelete string `yaml:"kept-delete"`
	// Required marks a pure virtual a Go subclass must define, because Qt
	// uses its result unchecked (QIconEngine::clone); the text says when Qt
	// calls it. NewXWith panics for a type that leaves it out.
	Required string `yaml:"required"`
}

func (o *overlay) skipped() bool {
	if o == nil || o.Skip == nil {
		return false
	}
	switch v := o.Skip.(type) {
	case bool:
		return v
	case string:
		return v != ""
	}
	return true
}

type overlayFile map[string]map[string]*overlay

func loadYAML(path string, into any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// overlayFor returns the merged overlay for a method: bare-name entries first,
// then a matching signature entry on top.
func (f overlayFile) overlayFor(class string, m *irMethod) (*overlay, []string) {
	entries := f[class]
	if entries == nil {
		return nil, nil
	}
	var used []string
	var out *overlay
	merge := func(o *overlay) {
		if o == nil {
			return
		}
		if out == nil {
			c := *o
			c.Args = map[string]string{}
			for k, v := range o.Args {
				c.Args[k] = v
			}
			out = &c
			return
		}
		for k, v := range o.Args {
			out.Args[k] = v
		}
		if o.Return != "" {
			out.Return = o.Return
		}
		if o.Name != "" {
			out.Name = o.Name
		}
		if o.Full != "" {
			out.Full = o.Full
		}
		if o.KeepDefs {
			out.KeepDefs = true
		}
		if o.NoFull != "" {
			out.NoFull = o.NoFull
		}
		if o.Skip != nil {
			out.Skip = o.Skip
		}
		if o.Error {
			out.Error = true
		}
		if o.ErrorMsg != "" {
			out.ErrorMsg = o.ErrorMsg
		}
		if o.Cpp != "" {
			out.Cpp = o.Cpp
		}
		if o.Created {
			out.Created = true
		}
		if o.Reviewed {
			out.Reviewed = true
		}
		if o.Test != "" {
			out.Test = o.Test
		}
		if o.Doc != "" {
			out.Doc = o.Doc
		}
		if o.ReleaseSlot != "" {
			out.ReleaseSlot = o.ReleaseSlot
		}
		if o.KeptDelete != "" {
			out.KeptDelete = o.KeptDelete
		}
		if o.Required != "" {
			out.Required = o.Required
		}
	}
	if o, ok := entries[m.Name]; ok {
		merge(o)
		used = append(used, m.Name)
	}
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !strings.Contains(k, "(") {
			continue
		}
		name := k[:strings.Index(k, "(")]
		if name != m.Name {
			continue
		}
		if sigMatches(k[len(name):], m) {
			merge(entries[k])
			used = append(used, k)
		}
	}
	return out, used
}

func sigMatches(sig string, m *irMethod) bool {
	sig = strings.TrimSpace(sig)
	constM := strings.HasSuffix(sig, "const")
	sig = strings.TrimSuffix(sig, "const")
	sig = strings.TrimSpace(sig)
	if !strings.HasPrefix(sig, "(") || !strings.HasSuffix(sig, ")") {
		return false
	}
	inner := sig[1 : len(sig)-1]
	var parts []string
	if strings.TrimSpace(inner) != "" {
		parts = splitTopLevel(inner)
	}
	if len(parts) != len(m.Params) {
		return false
	}
	if constM && !m.Const {
		return false
	}
	for i, p := range parts {
		if !typeMatches(p, m.Params[i].Type) {
			return false
		}
	}
	return true
}

// typeMatches compares an overlay type with an IR type, accepting the
// spelling, the canonical form, or a class-qualified spelling.
func typeMatches(want string, t irType) bool {
	wb, wc, wp, wr, _ := parseCpp(want)
	for _, have := range []string{t.Spelling, t.Canonical} {
		hb, hc, hp, hr, _ := parseCpp(have)
		if wc != hc || wp != hp || wr != hr {
			continue
		}
		wb, hb := strings.ReplaceAll(wb, " ", ""), strings.ReplaceAll(hb, " ", "")
		if wb == hb || strings.HasSuffix(wb, "::"+hb) {
			return true
		}
	}
	return false
}

func splitTopLevel(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, c := range s {
		switch c {
		case '<', '(':
			depth++
		case '>', ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(s[start:]))
}
