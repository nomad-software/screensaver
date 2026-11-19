package rt

import "testing"

// Stand-ins with the layout of generated code: a wrapper starts with a Ref,
// a derived wrapper embeds its base, a Base type holds a pointer to its
// wrapper and declares the protected methods.
type udWrapper struct{ quteRef Ref }

func (w *udWrapper) SizeHint() int { return 0 }
func (w *udWrapper) Paint()        {}

type udDerived struct{ udWrapper }

type udBase struct{ *udDerived }

func (b *udBase) Protected() {}

type userDirect struct {
	udBase
	n int
}

func (u *userDirect) SizeHint() int { return 1 }

type userNested struct{ userDirect }

type userDeep struct {
	userNested
	extra int
}

type userByPointer struct{ *userDirect }

type userNothing struct{ udBase }

type userValueReceiver struct{ udBase }

func (u userValueReceiver) Paint() {}

// An override counts wherever in the embedding the user declared it; what a
// type only inherits from the generated wrapper or Base never does, or a
// public virtual would dispatch to itself.
func TestUserDefined(t *testing.T) {
	for _, c := range []struct {
		name   string
		v      any
		method string
		want   bool
	}{
		{"declared on the type", &userDirect{}, "SizeHint", true},
		{"declared on an embedded user type", &userNested{}, "SizeHint", true},
		{"two embeddings deep", &userDeep{}, "SizeHint", true},
		{"embedded by pointer", &userByPointer{&userDirect{}}, "SizeHint", true},
		{"value receiver", &userValueReceiver{}, "Paint", true},
		{"only the wrapper's", &userNothing{}, "SizeHint", false},
		{"the wrapper's, next to an override", &userDirect{}, "Paint", false},
		{"the wrapper's, through user types", &userDeep{}, "Paint", false},
		{"a protected method of the Base", &userNothing{}, "Protected", false},
		{"no such method", &userDirect{}, "Missing", false},
		{"nil", nil, "SizeHint", false},
	} {
		if got := UserDefined(c.v, c.method); got != c.want {
			t.Errorf("%s: UserDefined(%T, %q) = %v, want %v", c.name, c.v, c.method, got, c.want)
		}
	}
}

// RequireOverride lets a type through that declares the virtual and panics,
// naming the type, the method and why Qt needs it, for one that does not.
func TestRequireOverride(t *testing.T) {
	vt := []Virtual{{Name: "SizeHint", Has: func(v any) bool { _, ok := v.(interface{ SizeHint() int }); return ok }}}
	RequireOverride(&userDirect{}, vt, "NewThingWith", "SizeHint", "it sizes the thing")
	defer func() {
		msg, _ := recover().(string)
		want := "NewThingWith: *rt.userNothing does not define SizeHint, which Qt needs: it sizes the thing; define SizeHint on the type"
		if msg != want {
			t.Errorf("panic %q\n want %q", msg, want)
		}
	}()
	RequireOverride(&userNothing{}, vt, "NewThingWith", "SizeHint", "it sizes the thing")
}
