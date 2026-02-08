package parser

import (
	"slices"
	"testing"
)

var testCases = []struct {
	desc    string
	kt      string
	pkg     string
	imports []string
	hasMain bool
}{
	{
		desc:    "empty",
		kt:      "",
		imports: []string{},
	},
	{
		desc: "simple",
		kt: `
import a.B
import c.D as E
	`,
		imports: []string{"a", "c"},
	},
	{
		desc: "stars",
		kt: `package a.b.c

import  d.y.* 
		`,
		pkg:     "a.b.c",
		imports: []string{"d.y"},
	},
	{
		desc: "line-comments",
		kt: `
package x // trailing comment
// import fake.Commented
import a.B // trailing
import c.D // trailing
import d.* // trailing
		`,
		pkg:     "x",
		imports: []string{"a", "c", "d"},
	},
	{
		desc: "value-classes",
		kt: `
@JvmInline
value class Password(private val s: String)
	`,
		imports: []string{},
	},
	{
		desc: "fun-interface",
		kt: `
package com.example

import com.example.other.SomeClass

fun interface MyAction {
    fun execute(): String
}
	`,
		pkg:     "com.example",
		imports: []string{"com.example.other"},
	},
	{
		desc: "fun-interface-with-main",
		kt: `
package com.example

import com.example.other.Dependency

fun interface Transformer {
    fun transform(input: String): String
}

fun main() {
    val t = Transformer { it.uppercase() }
    println(t.transform("hello"))
}
	`,
		pkg:     "com.example",
		imports: []string{"com.example.other"},
		hasMain: true,
	},
	{
		desc: "fun-interface-no-imports",
		kt: `
fun interface Runnable {
    fun run()
}
	`,
		imports: []string{},
	},
	{
		desc:    "main-simple",
		kt:      "fun main() {}",
		imports: []string{},
		hasMain: true,
	},
	{
		desc: "main-with-package",
		kt: `
package my.demo
fun main() {}
	`,
		pkg:     "my.demo",
		imports: []string{},
		hasMain: true,
	},
	{
		desc: "main-with-args",
		kt: `
fun main(args: Array<String>) {
    println(args)
}
	`,
		imports: []string{},
		hasMain: true,
	},
	{
		desc: "main-with-imports",
		kt: `
package my.demo
import kotlin.text.*
fun main() {}
	`,
		pkg:     "my.demo",
		imports: []string{"kotlin.text"},
		hasMain: true,
	},
	{
		desc: "suspend-fun-main",
		kt: `
import kotlinx.coroutines.*

suspend fun main() {
    println("hello from coroutines")
}
	`,
		imports: []string{"kotlinx.coroutines"},
		hasMain: true,
	},
	{
		desc: "suspend-fun-main-with-args",
		kt: `
suspend fun main(args: Array<String>) {
    delay(1000)
}
	`,
		imports: []string{},
		hasMain: true,
	},
	{
		desc: "no-main-regular-fun",
		kt: `
fun helper() {}
fun process(x: Int): String { return x.toString() }
	`,
		imports: []string{},
		hasMain: false,
	},
	{
		desc: "no-main-nested-in-object",
		kt: `
package test.lib

object Constants {
    const val NAME = "test"

    fun main() {
        print("main!")
    }
}

fun nonMain() {}
	`,
		pkg:     "test.lib",
		imports: []string{},
		hasMain: false,
	},
	{
		desc: "no-main-nested-in-class",
		kt: `
class MyApp {
    fun main() {
        println("not a real main")
    }
}
	`,
		imports: []string{},
		hasMain: false,
	},
	{
		desc: "no-main-in-companion-object",
		kt: `
class Application {
    companion object {
        @JvmStatic
        fun main(args: Array<String>) {
            println("companion main")
        }
    }
}
	`,
		imports: []string{},
		hasMain: false,
	},
	{
		desc: "main-after-class",
		kt: `
class Helper {
    fun doWork() {}
}

fun main() {
    Helper().doWork()
}
	`,
		imports: []string{},
		hasMain: true,
	},
	{
		desc: "single-segment-import-skipped",
		kt: `
import a
import b.C
	`,
		imports: []string{"b"},
	},
	{
		desc: "raw-string-false-imports",
		kt: `
package com.example

val query = """
    import fake.Package
    fun main() {}
"""

import real.Dep
	`,
		pkg:     "com.example",
		imports: []string{"real"},
	},
	{
		desc:    "private-fun-main",
		kt:      "private fun main() {}",
		imports: []string{},
		hasMain: true,
	},
	{
		desc:    "internal-fun-main",
		kt:      "internal fun main() {}",
		imports: []string{},
		hasMain: true,
	},
	{
		desc: "multi-line-block-comment",
		kt: `
package com.example

/*
import fake.Commented
import another.Commented
*/

import real.Dependency

class Foo
	`,
		pkg:     "com.example",
		imports: []string{"real"},
	},
	{
		desc: "nested-block-comments",
		kt: `
package com.example

/* outer /* inner */ still a comment */

import real.Dep
	`,
		pkg:     "com.example",
		imports: []string{"real"},
	},
	{
		desc: "multiple-modifiers-main",
		kt: `
public suspend fun main() {
    println("hello")
}
	`,
		imports: []string{},
		hasMain: true,
	},
	{
		desc: "duplicate-package-imports",
		kt: `
import com.example.Foo
import com.example.Bar
	`,
		imports: []string{"com.example", "com.example"},
	},
	{
		desc: "triple-quote-in-line-comment",
		kt: `
package com.example

val x = 1 // some """ example
import real.Dep
	`,
		pkg:     "com.example",
		imports: []string{"real"},
	},
	{
		desc: "block-comment-open-in-line-comment",
		kt: `
package com.example

val x = 1 // see /*
import real.Dep
	`,
		pkg:     "com.example",
		imports: []string{"real"},
	},
	{
		desc: "main-with-space-before-paren",
		kt: `
fun main () {
    println("hello")
}
	`,
		imports: []string{},
		hasMain: true,
	},
	{
		desc: "block-comment-marker-inside-string",
		kt: `
package com.example

val pattern = "/* start"
import real.Dep
val end = "end */"
	`,
		pkg:     "com.example",
		imports: []string{"real"},
	},
	{
		desc: "block-comment-extra-close",
		kt: `
package com.example

/* comment */ */ stray close

/* second block
   still a comment
*/

import real.Dep
	`,
		pkg:     "com.example",
		imports: []string{"real"},
	},
	{
		desc: "top-level-main-with-indentation",
		kt: `
package sample
    fun main() {}
`,
		pkg:     "sample",
		imports: []string{},
		hasMain: true,
	},
	{
		desc: "nested-main-at-column-zero",
		kt: `
class App {
fun main() {}
}
`,
		imports: []string{},
		hasMain: false,
	},
	{
		desc:    "semicolon-separated-declarations",
		kt:      `package a.b; import c.D; fun main() {}`,
		pkg:     "a.b",
		imports: []string{"c"},
		hasMain: true,
	},
	{
		desc: "annotation-before-main",
		kt: `
@JvmName("entry") fun main() {}
`,
		imports: []string{},
		hasMain: true,
	},
	{
		desc:    "escaped-keywords-not-declarations",
		kt:      "val `package` = 1\nval `import` = 2\nval `fun` = 3\n",
		imports: []string{},
	},
	{
		desc:    "escaped-identifiers-in-package-and-import",
		kt:      "package com.`when`\nimport com.`when`.Service\n",
		pkg:     "com.when",
		imports: []string{"com.when"},
	},
}

func TestParse(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			res := Parse([]byte(tc.kt))

			if res.Package != tc.pkg {
				t.Errorf("Package:\n  got:  %q\n  want: %q\nkotlin code:\n%s", res.Package, tc.pkg, tc.kt)
			}

			if !slices.Equal(res.Imports, tc.imports) {
				t.Errorf("Imports:\n  got:  %#v\n  want: %#v\nkotlin code:\n%s", res.Imports, tc.imports, tc.kt)
			}

			if res.HasMain != tc.hasMain {
				t.Errorf("HasMain:\n  got:  %v\n  want: %v\nkotlin code:\n%s", res.HasMain, tc.hasMain, tc.kt)
			}
		})
	}
}
