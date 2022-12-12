// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
)

var debug = flag.Bool("debug", false, "")

func main() {
	flag.Parse()

	// Generate table.go.
	{
		w := &bytes.Buffer{}
		w.WriteString(header)
		w.WriteString(decodeHeaderComment)
		writeDecodeTable(w, build(modeCodes[:], 0), "modeDecodeTable",
			"// modeDecodeTable represents Table 1 and the End-of-Line code.\n")
		writeDecodeTable(w, build(whiteCodes[:], 0), "whiteDecodeTable",
			"// whiteDecodeTable represents Tables 2 and 3 for a white run.\n")
		writeDecodeTable(w, build(blackCodes[:], 0), "blackDecodeTable",
			"// blackDecodeTable represents Tables 2 and 3 for a black run.\n")
		writeMaxCodeLength(w, modeCodes[:], whiteCodes[:], blackCodes[:])
		w.WriteString(encodeHeaderComment)
		w.WriteString(bitStringTypeDef)
		writeEncodeTable(w, modeCodes[:], "modeEncodeTable",
			"// modeEncodeTable represents Table 1 and the End-of-Line code.\n")
		writeEncodeTable(w, whiteCodes[:64], "whiteEncodeTable2",
			"// whiteEncodeTable2 represents Table 2 for a white run.\n")
		writeEncodeTable(w, whiteCodes[64:], "whiteEncodeTable3",
			"// whiteEncodeTable3 represents Table 3 for a white run.\n")
		writeEncodeTable(w, blackCodes[:64], "blackEncodeTable2",
			"// blackEncodeTable2 represents Table 2 for a black run.\n")
		writeEncodeTable(w, blackCodes[64:], "blackEncodeTable3",
			"// blackEncodeTable3 represents Table 3 for a black run.\n")
		finish(w, "table.go")
	}

	// Generate table_test.go.
	{
		w := &bytes.Buffer{}
		w.WriteString(header)
		finish(w, "table_test.go")
	}
}

const header = `// generated by "go run gen.go". DO NOT EDIT.

package ccitt

`

const decodeHeaderComment = `
// Each decodeTable is represented by an array of [2]int16's: a binary tree.
// Each array element (other than element 0, which means invalid) is a branch
// node in that tree. The root node is always element 1 (the second element).
//
// To walk the tree, look at the next bit in the bit stream, using it to select
// the first or second element of the [2]int16. If that int16 is 0, we have an
// invalid code. If it is positive, go to that branch node. If it is negative,
// then we have a leaf node, whose value is the bitwise complement (the ^
// operator) of that int16.
//
// Comments above each decodeTable also show the same structure visually. The
// "b123" lines show the 123'rd branch node. The "=XXXXX" lines show an invalid
// code. The "=v1234" lines show a leaf node with value 1234. When reading the
// bit stream, a 0 or 1 bit means to go up or down, as you move left to right.
//
// For example, in modeDecodeTable, branch node b005 is three steps up from the
// root node, meaning that we have already seen "000". If the next bit is "0"
// then we move to branch node b006. Otherwise, the next bit is "1", and we
// move to the leaf node v0000 (also known as the modePass constant). Indeed,
// the bits that encode modePass are "0001".
//
// Tables 1, 2 and 3 come from the "ITU-T Recommendation T.6: FACSIMILE CODING
// SCHEMES AND CODING CONTROL FUNCTIONS FOR GROUP 4 FACSIMILE APPARATUS"
// specification:
//
// https://www.itu.int/rec/dologin_pub.asp?lang=e&id=T-REC-T.6-198811-I!!PDF-E&type=items


`

const encodeHeaderComment = `
// Each encodeTable is represented by an array of bitStrings.


`

type node struct {
	children    [2]*node
	val         uint32
	branchIndex int32
}

func (n *node) isBranch() bool {
	return (n != nil) && ((n.children[0] != nil) || (n.children[1] != nil))
}

func (n *node) String() string {
	if n == nil {
		return "0"
	}
	if n.branchIndex > 0 {
		return fmt.Sprintf("%d", n.branchIndex)
	}
	return fmt.Sprintf("^%d", n.val)
}

func build(codes []code, prefixLen int) *node {
	if len(codes) == 0 {
		return nil
	}

	if prefixLen == len(codes[0].str) {
		if len(codes) != 1 {
			panic("ambiguous codes")
		}
		return &node{
			val: codes[0].val,
		}
	}

	childrenCodes := [2][]code{}
	for _, code := range codes {
		bit := code.str[prefixLen] & 1
		childrenCodes[bit] = append(childrenCodes[bit], code)
	}
	return &node{
		children: [2]*node{
			build(childrenCodes[0], prefixLen+1),
			build(childrenCodes[1], prefixLen+1),
		},
	}
}

func writeDecodeTable(w *bytes.Buffer, root *node, varName, comment string) {
	assignBranchIndexes(root)

	w.WriteString(comment)
	w.WriteString("//\n")
	writeComment(w, root, "  ", false)
	fmt.Fprintf(w, "var %s = [...][2]int16{\n", varName)
	fmt.Fprintf(w, "0: {0, 0},\n")

	// Walk the tree in breadth-first order.
	for queue := []*node{root}; len(queue) > 0; {
		n := queue[0]
		queue = queue[1:]

		if n.isBranch() {
			fmt.Fprintf(w, "%d: {%v, %v},\n", n.branchIndex, n.children[0], n.children[1])
			queue = append(queue, n.children[0], n.children[1])
		}
	}

	fmt.Fprintf(w, "}\n\n")
}

const bitStringTypeDef = `
// bitString is a pair of uint32 values representing a bit code.
// The nBits low bits of bits make up the actual bit code.
// Eg. bitString{0x0004, 8} represents the bitcode "00000100".
type bitString struct {
	bits  uint32
	nBits uint32
}

`

func writeEncodeTable(w *bytes.Buffer, codes []code, varName, comment string) {
	w.WriteString(comment)
	fmt.Fprintf(w, "var %s = [...]bitString{\n", varName)
	for i, code := range codes {
		s := code.str
		n := uint32(len(s))
		c := uint32(0)
		for j := uint32(0); j < n; j++ {
			c |= uint32(s[j]&1) << (n - j - 1)
		}
		fmt.Fprintf(w, "%d: {0x%04x, %v}, // %q \n", i, c, n, s)
	}
	fmt.Fprintf(w, "}\n\n")
}

func assignBranchIndexes(root *node) {
	// 0 is reserved for an invalid value.
	branchIndex := int32(1)

	// Walk the tree in breadth-first order.
	for queue := []*node{root}; len(queue) > 0; {
		n := queue[0]
		queue = queue[1:]

		if n.isBranch() {
			n.branchIndex = branchIndex
			branchIndex++
			queue = append(queue, n.children[0], n.children[1])
		}
	}
}

func writeComment(w *bytes.Buffer, n *node, prefix string, down bool) {
	if n.isBranch() {
		prefixUp := prefix[:len(prefix)-2] + "  | "
		prefixDown := prefix + "| "
		if down {
			prefixUp, prefixDown = prefixDown, prefixUp
		}

		writeComment(w, n.children[0], prefixUp, false)
		defer writeComment(w, n.children[1], prefixDown, true)

		fmt.Fprintf(w, "//\tb%03d ", n.branchIndex)
	} else {
		fmt.Fprintf(w, "//\t     ")
	}

	w.WriteString(prefix[:len(prefix)-2])

	if n == nil {
		fmt.Fprintf(w, "+=XXXXX\n")
		return
	}
	if !n.isBranch() {
		fmt.Fprintf(w, "+=v%04d\n", n.val)
		return
	}
	w.WriteString("+-+\n")
}

func writeMaxCodeLength(w *bytes.Buffer, codesList ...[]code) {
	maxCodeLength := 0
	for _, codes := range codesList {
		for _, code := range codes {
			if n := len(code.str); maxCodeLength < n {
				maxCodeLength = n
			}
		}
	}
	fmt.Fprintf(w, "const maxCodeLength = %d\n\n", maxCodeLength)
}

func finish(w *bytes.Buffer, filename string) {
	copyPaste(w, filename)
	if *debug {
		os.Stdout.Write(w.Bytes())
		return
	}
	out, err := format.Source(w.Bytes())
	if err != nil {
		log.Fatalf("format.Source: %v", err)
	}
	if err := ioutil.WriteFile(filename, out, 0660); err != nil {
		log.Fatalf("ioutil.WriteFile: %v", err)
	}
}

func copyPaste(w *bytes.Buffer, filename string) {
	b, err := ioutil.ReadFile("gen.go")
	if err != nil {
		log.Fatalf("ioutil.ReadFile: %v", err)
	}
	begin := []byte("\n// COPY PASTE " + filename + " BEGIN\n\n")
	end := []byte("\n// COPY PASTE " + filename + " END\n\n")

	for len(b) > 0 {
		i := bytes.Index(b, begin)
		if i < 0 {
			break
		}
		b = b[i:]

		j := bytes.Index(b, end)
		if j < 0 {
			break
		}
		j += len(end)

		w.Write(b[:j])
		b = b[j:]
	}
}

// COPY PASTE table.go BEGIN

const (
	modePass = iota // Pass
	modeH           // Horizontal
	modeV0          // Vertical-0
	modeVR1         // Vertical-Right-1
	modeVR2         // Vertical-Right-2
	modeVR3         // Vertical-Right-3
	modeVL1         // Vertical-Left-1
	modeVL2         // Vertical-Left-2
	modeVL3         // Vertical-Left-3
	modeExt         // Extension
)

// COPY PASTE table.go END

// The data that is the rest of this file is taken from Tables 1, 2 and 3 from
// the "ITU-T Recommendation T.6" spec.

// COPY PASTE table_test.go BEGIN

type code struct {
	val uint32
	str string
}

var modeCodes = []code{
	{modePass, "0001"},
	{modeH, "001"},
	{modeV0, "1"},
	{modeVR1, "011"},
	{modeVR2, "000011"},
	{modeVR3, "0000011"},
	{modeVL1, "010"},
	{modeVL2, "000010"},
	{modeVL3, "0000010"},
	{modeExt, "0000001"},
}

var whiteCodes = []code{
	// Terminating codes (0-63).
	{0x0000, "00110101"},
	{0x0001, "000111"},
	{0x0002, "0111"},
	{0x0003, "1000"},
	{0x0004, "1011"},
	{0x0005, "1100"},
	{0x0006, "1110"},
	{0x0007, "1111"},
	{0x0008, "10011"},
	{0x0009, "10100"},
	{0x000A, "00111"},
	{0x000B, "01000"},
	{0x000C, "001000"},
	{0x000D, "000011"},
	{0x000E, "110100"},
	{0x000F, "110101"},
	{0x0010, "101010"},
	{0x0011, "101011"},
	{0x0012, "0100111"},
	{0x0013, "0001100"},
	{0x0014, "0001000"},
	{0x0015, "0010111"},
	{0x0016, "0000011"},
	{0x0017, "0000100"},
	{0x0018, "0101000"},
	{0x0019, "0101011"},
	{0x001A, "0010011"},
	{0x001B, "0100100"},
	{0x001C, "0011000"},
	{0x001D, "00000010"},
	{0x001E, "00000011"},
	{0x001F, "00011010"},
	{0x0020, "00011011"},
	{0x0021, "00010010"},
	{0x0022, "00010011"},
	{0x0023, "00010100"},
	{0x0024, "00010101"},
	{0x0025, "00010110"},
	{0x0026, "00010111"},
	{0x0027, "00101000"},
	{0x0028, "00101001"},
	{0x0029, "00101010"},
	{0x002A, "00101011"},
	{0x002B, "00101100"},
	{0x002C, "00101101"},
	{0x002D, "00000100"},
	{0x002E, "00000101"},
	{0x002F, "00001010"},
	{0x0030, "00001011"},
	{0x0031, "01010010"},
	{0x0032, "01010011"},
	{0x0033, "01010100"},
	{0x0034, "01010101"},
	{0x0035, "00100100"},
	{0x0036, "00100101"},
	{0x0037, "01011000"},
	{0x0038, "01011001"},
	{0x0039, "01011010"},
	{0x003A, "01011011"},
	{0x003B, "01001010"},
	{0x003C, "01001011"},
	{0x003D, "00110010"},
	{0x003E, "00110011"},
	{0x003F, "00110100"},

	// Make-up codes between 64 and 1728.
	{0x0040, "11011"},
	{0x0080, "10010"},
	{0x00C0, "010111"},
	{0x0100, "0110111"},
	{0x0140, "00110110"},
	{0x0180, "00110111"},
	{0x01C0, "01100100"},
	{0x0200, "01100101"},
	{0x0240, "01101000"},
	{0x0280, "01100111"},
	{0x02C0, "011001100"},
	{0x0300, "011001101"},
	{0x0340, "011010010"},
	{0x0380, "011010011"},
	{0x03C0, "011010100"},
	{0x0400, "011010101"},
	{0x0440, "011010110"},
	{0x0480, "011010111"},
	{0x04C0, "011011000"},
	{0x0500, "011011001"},
	{0x0540, "011011010"},
	{0x0580, "011011011"},
	{0x05C0, "010011000"},
	{0x0600, "010011001"},
	{0x0640, "010011010"},
	{0x0680, "011000"},
	{0x06C0, "010011011"},

	// Make-up codes between 1792 and 2560.
	{0x0700, "00000001000"},
	{0x0740, "00000001100"},
	{0x0780, "00000001101"},
	{0x07C0, "000000010010"},
	{0x0800, "000000010011"},
	{0x0840, "000000010100"},
	{0x0880, "000000010101"},
	{0x08C0, "000000010110"},
	{0x0900, "000000010111"},
	{0x0940, "000000011100"},
	{0x0980, "000000011101"},
	{0x09C0, "000000011110"},
	{0x0A00, "000000011111"},
}

var blackCodes = []code{
	// Terminating codes (0-63).
	{0x0000, "0000110111"},
	{0x0001, "010"},
	{0x0002, "11"},
	{0x0003, "10"},
	{0x0004, "011"},
	{0x0005, "0011"},
	{0x0006, "0010"},
	{0x0007, "00011"},
	{0x0008, "000101"},
	{0x0009, "000100"},
	{0x000A, "0000100"},
	{0x000B, "0000101"},
	{0x000C, "0000111"},
	{0x000D, "00000100"},
	{0x000E, "00000111"},
	{0x000F, "000011000"},
	{0x0010, "0000010111"},
	{0x0011, "0000011000"},
	{0x0012, "0000001000"},
	{0x0013, "00001100111"},
	{0x0014, "00001101000"},
	{0x0015, "00001101100"},
	{0x0016, "00000110111"},
	{0x0017, "00000101000"},
	{0x0018, "00000010111"},
	{0x0019, "00000011000"},
	{0x001A, "000011001010"},
	{0x001B, "000011001011"},
	{0x001C, "000011001100"},
	{0x001D, "000011001101"},
	{0x001E, "000001101000"},
	{0x001F, "000001101001"},
	{0x0020, "000001101010"},
	{0x0021, "000001101011"},
	{0x0022, "000011010010"},
	{0x0023, "000011010011"},
	{0x0024, "000011010100"},
	{0x0025, "000011010101"},
	{0x0026, "000011010110"},
	{0x0027, "000011010111"},
	{0x0028, "000001101100"},
	{0x0029, "000001101101"},
	{0x002A, "000011011010"},
	{0x002B, "000011011011"},
	{0x002C, "000001010100"},
	{0x002D, "000001010101"},
	{0x002E, "000001010110"},
	{0x002F, "000001010111"},
	{0x0030, "000001100100"},
	{0x0031, "000001100101"},
	{0x0032, "000001010010"},
	{0x0033, "000001010011"},
	{0x0034, "000000100100"},
	{0x0035, "000000110111"},
	{0x0036, "000000111000"},
	{0x0037, "000000100111"},
	{0x0038, "000000101000"},
	{0x0039, "000001011000"},
	{0x003A, "000001011001"},
	{0x003B, "000000101011"},
	{0x003C, "000000101100"},
	{0x003D, "000001011010"},
	{0x003E, "000001100110"},
	{0x003F, "000001100111"},

	// Make-up codes between 64 and 1728.
	{0x0040, "0000001111"},
	{0x0080, "000011001000"},
	{0x00C0, "000011001001"},
	{0x0100, "000001011011"},
	{0x0140, "000000110011"},
	{0x0180, "000000110100"},
	{0x01C0, "000000110101"},
	{0x0200, "0000001101100"},
	{0x0240, "0000001101101"},
	{0x0280, "0000001001010"},
	{0x02C0, "0000001001011"},
	{0x0300, "0000001001100"},
	{0x0340, "0000001001101"},
	{0x0380, "0000001110010"},
	{0x03C0, "0000001110011"},
	{0x0400, "0000001110100"},
	{0x0440, "0000001110101"},
	{0x0480, "0000001110110"},
	{0x04C0, "0000001110111"},
	{0x0500, "0000001010010"},
	{0x0540, "0000001010011"},
	{0x0580, "0000001010100"},
	{0x05C0, "0000001010101"},
	{0x0600, "0000001011010"},
	{0x0640, "0000001011011"},
	{0x0680, "0000001100100"},
	{0x06C0, "0000001100101"},

	// Make-up codes between 1792 and 2560.
	{0x0700, "00000001000"},
	{0x0740, "00000001100"},
	{0x0780, "00000001101"},
	{0x07C0, "000000010010"},
	{0x0800, "000000010011"},
	{0x0840, "000000010100"},
	{0x0880, "000000010101"},
	{0x08C0, "000000010110"},
	{0x0900, "000000010111"},
	{0x0940, "000000011100"},
	{0x0980, "000000011101"},
	{0x09C0, "000000011110"},
	{0x0A00, "000000011111"},
}

// COPY PASTE table_test.go END

// This final comment makes the "END" above be followed by "\n\n".
