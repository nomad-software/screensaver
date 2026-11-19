package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVet checks every "// want rule" comment in testdata/bad is reported,
// and nothing else is.
func TestVet(t *testing.T) {
	got, err := vetFindings([]string{"./testdata/bad"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]string{}
	f, err := os.Open(filepath.Join("testdata", "bad", "bad.go"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		if i := strings.Index(sc.Text(), "// want "); i >= 0 {
			want[line] = strings.TrimSpace(sc.Text()[i+len("// want "):])
		}
	}
	for _, g := range got {
		if want[g.pos.Line] != g.rule {
			t.Errorf("unexpected %s at line %d: %s", g.rule, g.pos.Line, g.msg)
		}
		delete(want, g.pos.Line)
	}
	for line, rule := range want {
		t.Errorf("missing %s at line %d", rule, line)
	}
}
