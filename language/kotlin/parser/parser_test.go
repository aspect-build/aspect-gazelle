package parser

import (
	"bytes"
	"encoding/gob"
	"sort"
	"testing"
)

var testCases = []struct {
	desc, kt string
	want     parseResultComparable
}{
	{
		desc: "import star",
		kt: `package a.b.c

import  x.y.z.* 
		`,
		want: parseResultComparable{
			File:    "stars.kt",
			Package: "a.b.c",
			Imports: []importComparable{
				{Identifier: "x.y.z", IsStar: true},
			},
		},
	},
	{
		desc: "aliased",
		kt: `package hey.there

import com.example.foo.Bar as MyBar
import com.example.foo.Bar as /*x*/MyBar2
`,
		want: parseResultComparable{
			File:    "aliased.kt",
			Package: "hey.there",
			Imports: []importComparable{
				{Identifier: "com.example.foo.Bar", Alias: "MyBar"},
				{Identifier: "com.example.foo.Bar", Alias: "MyBar2"},
			},
		},
	},
	{
		desc: "empty",
		kt:   "",
		want: parseResultComparable{
			File:    "empty.kt",
			Package: "",
			Imports: []importComparable{},
		},
	},
	{
		desc: "simple",
		kt: `
import a.B
import c.D as E
	`,
		want: parseResultComparable{
			File:    "simple.kt",
			Package: "",
			Imports: []importComparable{
				{Identifier: "a.B"},
				{Identifier: "c.D", Alias: "E"},
			},
		},
	},
	{
		desc: "stars",
		kt: `package a.b.c

import  d.y.* 
		`,
		want: parseResultComparable{
			File:    "stars.kt",
			Package: "a.b.c",
			Imports: []importComparable{
				{Identifier: "d.y", IsStar: true},
			},
		},
	},
	{
		desc: "comments",
		kt: `
/*dlfkj*/package /*dlfkj*/ x // x
//z
import a.B // y
//z

/* asdf */ import /* asdf */ c.D // w
import /* fdsa */ d/* asdf */.* // w
				`,
		want: parseResultComparable{
			File:    "comments.kt",
			Package: "x",
			Imports: []importComparable{
				{Identifier: "a.B"},
				{Identifier: "c.D"},
				{Identifier: "d", IsStar: true},
			},
		},
	},
	// Fun interfaces (SAM): https://github.com/fwcd/tree-sitter-kotlin/issues/87
	{
		desc: "fun-interface",
		kt: `package com.example

import com.example.dep.Foo

fun interface MyHandler {
    fun handle(value: String): Boolean
}
`,
		want: parseResultComparable{
			File:    "handler.kt",
			Package: "com.example",
			Imports: []importComparable{
				{Identifier: "com.example.dep.Foo"},
			},
			TopLevelIdentifiers: []string{
				"MyHandler",
			},
		},
	},
	{
		desc: "value-classes",
		kt: `
@JvmInline
value class Password(private val s: String)
	`,
		want: parseResultComparable{
			File:    "simple.kt",
			Package: "",
			Imports: []importComparable{},
			TopLevelIdentifiers: []string{
				"Password",
			},
		},
	},
	{
		desc: "multiple top level objects",
		kt: `
@JvmInline
value class Password(private val s: String)

interface Inter {}

data class Point(val x: Int)

fun Thing.method(): Int = 5

fun fn(): Int = 5

var pi = 3.14

typealias AliasedInt = Int
	`,
		want: parseResultComparable{
			File:    "simple.kt",
			Package: "",
			TopLevelIdentifiers: []string{
				"AliasedInt",
				"fn",
				"Inter",
				"method",
				"Password",
				"Point",
				"pi",
			},
		},
	},
}

func TestTreesitterParser(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			res, err := NewParser().Parse(tc.want.File, []byte(tc.kt))
			if err != nil {
				t.Errorf("Errors parsing %q: %v", tc.want.File, err)
			}

			tc.want.sort()
			got := makeComparable(res)
			if !equalParseResultComparable(tc.want, got) {
				t.Errorf("unexpected results:\nwant: %#v\ngot:  %#v\n", tc.want, got)
			}
		})
	}
}

// TestParseResultGobRoundTrip guards against caching corruption: ParseResult is
// gob-encoded for the on-disk cache, and gob ignores unexported fields, so the
// Identifier/SimpleIdentifier/ImportStatement types must keep their fields exported.
func TestParseResultGobRoundTrip(t *testing.T) {
	src := `package com.example

import a.B
import c.D as E
import x.y.*

fun main() {}

class Widget
typealias Count = Int
`
	res, err := NewParser().Parse("gob.kt", []byte(src))
	if err != nil {
		t.Fatalf("Errors parsing: %v", err)
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(res); err != nil {
		t.Fatalf("gob encode failed: %v", err)
	}

	var decoded ParseResult
	if err := gob.NewDecoder(&buf).Decode(&decoded); err != nil {
		t.Fatalf("gob decode failed: %v", err)
	}

	want := makeComparable(res)
	want.sort()
	got := makeComparable(decoded)
	got.sort()
	if !equalParseResultComparable(want, got) {
		t.Errorf("ParseResult changed across gob round-trip:\nwant: %#v\ngot:  %#v", want, got)
	}
}

func equalParseResultComparable(a, b parseResultComparable) bool {
	if a.File != b.File || a.Package != b.Package || a.HasMain != b.HasMain {
		return false
	}
	if len(a.TopLevelIdentifiers) != len(b.TopLevelIdentifiers) {
		return false
	}
	for i, v := range a.TopLevelIdentifiers {
		if v != b.TopLevelIdentifiers[i] {
			return false
		}
	}
	if len(a.Imports) != len(b.Imports) {
		return false
	}
	for i, v := range a.Imports {
		if v != b.Imports[i] {
			return false
		}
	}
	return true
}

func TestNewSimpleIdentifier(t *testing.T) {
	valid := map[string]string{
		"Foo":      "Foo",
		"_under":   "_under",
		"a1b2":     "a1b2",
		"über":     "über",
		"`Foo`":    "Foo", // backticks around a valid identifier are normalized away
		"`object`": "object",
	}
	for in, want := range valid {
		si, err := NewSimpleIdentifier(in)
		if err != nil {
			t.Errorf("NewSimpleIdentifier(%q) unexpected error: %v", in, err)
			continue
		}
		if si.Literal != want {
			t.Errorf("NewSimpleIdentifier(%q).Literal = %q, want %q", in, si.Literal, want)
		}
	}

	invalid := []string{
		"",         // empty
		"1abc",     // leading digit
		"a-b",      // hyphen
		"a b",      // space
		"a.b",      // dotted path is not a single segment
		"`my var`", // backticked but inner is not a valid unquoted identifier
		"`",        // lone backtick must not panic
	}
	for _, in := range invalid {
		if si, err := NewSimpleIdentifier(in); err == nil {
			t.Errorf("NewSimpleIdentifier(%q) = %q, want error", in, si.Literal)
		}
	}
}

func TestMainDetection(t *testing.T) {
	t.Run("main detection", func(t *testing.T) {
		res, err := NewParser().Parse("main.kt", []byte("fun main() {}"))
		if err != nil {
			t.Errorf("Parse error: %v", err)
		}
		if !res.HasMain {
			t.Errorf("main method should be detected")
		}

		res, err = NewParser().Parse("x.kt", []byte(`
package my.demo
fun main() {}
		`))
		if err != nil {
			t.Errorf("Parse error: %v", err)
		}
		if !res.HasMain {
			t.Errorf("main method should be detected with package")
		}

		res, err = NewParser().Parse("x.kt", []byte(`
package my.demo
import kotlin.text.*
fun main() {}
		`))
		if err != nil {
			t.Errorf("Parse error: %v", err)
		}
		if !res.HasMain {
			t.Errorf("main method should be detected with imports")
		}
	})
}

type parseResultComparable struct {
	File                string
	Imports             []importComparable
	Package             string
	HasMain             bool
	TopLevelIdentifiers []string
}

func (pr *parseResultComparable) sort() {
	sort.Strings(pr.TopLevelIdentifiers)
}

type importComparable struct {
	Identifier string
	IsStar     bool
	Alias      string
}

func makeComparable(result ParseResult) parseResultComparable {
	var topLevelIds []string
	for _, id := range result.TopLevelIdentifiers {
		topLevelIds = append(topLevelIds, id.Normalize().Literal)
	}
	sort.Strings(topLevelIds)

	comparable := parseResultComparable{
		File:                result.File,
		Package:             result.Package.Literal(),
		HasMain:             result.HasMain,
		TopLevelIdentifiers: topLevelIds,
	}

	for _, imp := range result.Imports {
		comparable.Imports = append(comparable.Imports, importComparable{
			Identifier: imp.Identifier.Literal(),
			IsStar:     imp.IsStarImport,
			Alias:      imp.Alias.Literal,
		})
	}

	return comparable
}
