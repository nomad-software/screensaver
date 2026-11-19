// Command qute is the companion tool for the qute Qt bindings.
//
//	qute vet [packages]     static ownership and threading checks
//	qute new app <dir>      write a known-good starter application
package main

import (
	"fmt"
	"os"
)

const usage = `usage:
  qute vet [packages]        check for ownership and threading mistakes (default ./...)
  qute new app <dir> [module] write main.go, a window, a behaviour test and a screenshot test
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "vet":
		os.Exit(vet(os.Args[2:]))
	case "new":
		if len(os.Args) < 4 || os.Args[2] != "app" {
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
		module := ""
		if len(os.Args) > 4 {
			module = os.Args[4]
		}
		if err := newApp(os.Args[3], module); err != nil {
			fmt.Fprintln(os.Stderr, "qute new:", err)
			os.Exit(1)
		}
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
}
