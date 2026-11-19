package qtcore_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qutetest"
)

// resolver answers entities a document uses without declaring them.
type resolver struct {
	qtcore.QXmlStreamEntityResolverBase
	asked []string
}

func (r *resolver) ResolveUndeclaredEntity(name string) string {
	r.asked = append(r.asked, name)
	return "resolved-" + name
}

func TestXMLStreams(t *testing.T) {
	qutetest.Run(t, func() {
		buf := qtcore.NewQBuffer(nil)
		buf.Open(qtcore.QIODeviceBaseReadWrite)
		w := qtcore.NewQXmlStreamWriterWithIODevice(buf)
		if w.Device() != buf.AsQIODevice() {
			t.Error("writer device")
		}
		w.WriteStartDocument()
		w.WriteStartElement("root")
		w.WriteNamespace("urn:x")
		w.WriteAttributeQualifiedName("id", "1")
		w.WriteAttribute(qtcore.NewQXmlStreamAttributeWithQualifiedName("kind", "test"))
		w.WriteTextElement("item", "a & b")
		w.WriteEmptyElement("empty")
		w.WriteEndElement()
		w.WriteEndDocument()
		if w.HasError() {
			t.Fatal("writer error")
		}
		doc := string(buf.Data())
		w.Delete() // the writer uses the buffer: it goes first
		buf.Delete()
		if !strings.Contains(doc, `<item>a &amp; b</item>`) {
			t.Errorf("document %s", doc)
		}

		r := qtcore.NewQXmlStreamReaderWithData(doc)
		var names []string
		var copyBuf = qtcore.NewQBuffer(nil)
		copyBuf.Open(qtcore.QIODeviceBaseReadWrite)
		cw := qtcore.NewQXmlStreamWriterWithIODevice(copyBuf)
		for !r.AtEnd() {
			r.ReadNext()
			cw.WriteCurrentToken(r)
			if r.IsStartElement() {
				names = append(names, r.Name())
				if r.Name() == "root" {
					a := r.Attributes()
					if a.Value("id") != "1" || !a.HasAttribute("kind") {
						t.Errorf("root attributes: id %q", a.Value("id"))
					}
					// the list part of QXmlStreamAttributes is hand-written
					var listed []string
					for i := range a.Size() {
						at := a.At(i)
						if at.Ownership() != qtcore.Owned {
							t.Errorf("attribute %d: %v, want Owned", i, at.Ownership())
						}
						listed = append(listed, at.Name()+"="+at.Value())
					}
					if len(listed) != 2 || listed[0] != "id=1" || !strings.HasPrefix(listed[1], "kind=") {
						t.Errorf("listed attributes %v", listed)
					}
					func() {
						defer func() {
							if msg := fmt.Sprint(recover()); !strings.Contains(msg, "index 2 out of range") {
								t.Errorf("At past the end: panic %q", msg)
							}
						}()
						a.At(a.Size())
					}()
					ns := r.NamespaceDeclarations()
					if len(ns) != 1 || ns[0].NamespaceUri() != "urn:x" {
						t.Errorf("namespaces %v", ns)
					}
				}
			}
		}
		if r.HasError() || !slices.Equal(names, []string{"root", "item", "empty"}) {
			t.Errorf("elements %v, error %q", names, r.ErrorString())
		}
		if !strings.Contains(string(copyBuf.Data()), "<item>a &amp; b</item>") {
			t.Errorf("copied document %s", copyBuf.Data())
		}
		cw.Delete()
		copyBuf.Delete()
		r.Delete()

		dtd := `<!DOCTYPE r [<!ENTITY e "x"><!NOTATION n SYSTEM "sys">]><r>&e;&u;</r>`
		r2 := qtcore.NewQXmlStreamReaderWithData(dtd)
		res := &resolver{}
		r2.SetEntityResolver(qtcore.NewQXmlStreamEntityResolverWith(res))
		if r2.EntityResolver() == nil {
			t.Error("resolver not set")
		}
		text := ""
		for !r2.AtEnd() {
			switch r2.ReadNext() {
			case qtcore.QXmlStreamReaderDTD:
				ents := r2.EntityDeclarations()
				if len(ents) != 1 || ents[0].Name() != "e" || ents[0].Value() != "x" {
					t.Errorf("entities %v", ents)
				}
				nots := r2.NotationDeclarations()
				if len(nots) != 1 || nots[0].Name() != "n" || nots[0].SystemId() != "sys" {
					t.Errorf("notations %v", nots)
				}
			case qtcore.QXmlStreamReaderCharacters:
				text += r2.Text()
			}
		}
		if text != "xresolved-u" || !slices.Equal(res.asked, []string{"u"}) {
			t.Errorf("text %q, resolver asked %v (%s)", text, res.asked, r2.ErrorString())
		}
		r2.Delete()

		attrs := qtcore.NewQXmlStreamAttributes()
		attrs.Append("a", "1")
		attrs.AppendWithValue("urn:x", "b", "2")
		if attrs.Value("a") != "1" || attrs.ValueWithName("urn:x", "b") != "2" {
			t.Error("attribute list")
		}
		nd := qtcore.NewQXmlStreamNamespaceDeclarationWithPrefix("p", "urn:p")
		if nd.Prefix() != "p" || nd.NamespaceUri() != "urn:p" {
			t.Error("namespace declaration")
		}
		if qtcore.NewQXmlStreamEntityDeclaration().Name() != "" || qtcore.NewQXmlStreamNotationDeclaration().Name() != "" {
			t.Error("empty declarations")
		}
	})
}

func TestCBOR(t *testing.T) {
	qutetest.Run(t, func() {
		buf := qtcore.NewQBuffer(nil)
		buf.Open(qtcore.QIODeviceBaseReadWrite)
		w := qtcore.NewQCborStreamWriterWithIODevice(buf)
		w.StartMapWithCount(2)
		w.AppendStr("n")
		w.AppendInt64(-7)
		w.AppendStr("list")
		w.StartArrayWithCount(2)
		w.AppendBool(true)
		w.AppendBytes([]byte{1, 2})
		w.EndArray()
		w.EndMap()
		data := buf.Data()
		w.Delete()
		buf.Delete()

		r := qtcore.NewQCborStreamReaderWithBytes(data)
		if !r.IsMap() || !r.EnterContainer() {
			t.Fatalf("not a map: %v", r.Type())
		}
		if r.ReadAllString() != "n" || r.ToInteger() != -7 {
			t.Error("first pair")
		}
		r.Next()
		if r.ReadAllString() != "list" || !r.IsArray() {
			t.Error("second key")
		}
		r.Delete()

		v := qtcore.QCborValueFromCborBytes(data)
		m := v.ToMap()
		if !v.IsMap() || m.Size() != 2 || m.ValueKey("n").ToInteger() != -7 {
			t.Errorf("value: %s", v.ToDiagnosticNotation())
		}
		list := m.ValueKey("list").ToArray()
		if list.Size() != 2 || !list.At(0).ToBool() || string(list.At(1).ToByteArray()) != "\x01\x02" {
			t.Errorf("list %s", m.ValueKey("list").ToDiagnosticNotation())
		}
		if string(v.ToCbor()) != string(data) {
			t.Error("round trip changed the encoding")
		}

		arr := qtcore.NewQCborArray()
		arr.Append(qtcore.NewQCborValueWithString("x"))
		arr.Append(qtcore.NewQCborValueWithInt(3))
		if got := qtcore.NewQCborValueWithCborArray(arr).ToDiagnosticNotation(); got != `["x", 3]` {
			t.Errorf("array %s", got)
		}
		fromMap := qtcore.QCborMapFromVariantMap(map[string]*qtcore.QVariant{"k": qtcore.NewQVariantOf("v")})
		if fromMap.ValueKey("k").ToString() != "v" || len(fromMap.Keys()) != 1 {
			t.Error("map from variants")
		}

		perr := qtcore.NewQCborParserError()
		qtcore.QCborValueFromCborBytesFull([]byte{0xff, 0xff}, perr)
		if perr.Error().C() == 0 || perr.ErrorString() == "" {
			t.Errorf("parser error %q", perr.ErrorString())
		}
		if perr.Error().ToString() == "" || perr.Error().ToString() == qtcore.NewQCborError().ToString() {
			t.Errorf("error text %q", perr.Error().ToString())
		}

		jerr := qtcore.NewQJsonParseError()
		qtcore.QJsonDocumentFromJsonFull([]byte("{bad"), jerr)
		if jerr.Error() == qtcore.QJsonParseErrorNoError || jerr.ErrorString() == "" || jerr.Offset() == 0 {
			t.Errorf("JSON error %v at %d: %q", jerr.Error(), jerr.Offset(), jerr.ErrorString())
		}
	})
}
