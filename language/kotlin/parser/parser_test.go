package parser

import (
	"slices"
	"testing"
)

var testCases = []struct {
	desc     string
	kt       string
	filename string
	pkg      string
	imports  []string
	hasMain  bool
}{
	{
		desc:     "empty",
		kt:       "",
		filename: "empty.kt",
		pkg:      "",
		imports:  []string{},
	},
	{
		desc: "simple",
		kt: `
import a.B
import c.D as E
	`,
		filename: "simple.kt",
		pkg:      "",
		imports:  []string{"a", "c"},
	},
	{
		desc: "stars",
		kt: `package a.b.c

import  d.y.* 
		`,
		filename: "stars.kt",
		pkg:      "a.b.c",
		imports:  []string{"d.y"},
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
		filename: "comments.kt",
		pkg:      "x",
		imports:  []string{"a", "c", "d"},
	},
	{
		desc: "value-classes",
		kt: `
@JvmInline
value class Password(private val s: String)
	`,
		filename: "simple.kt",
		pkg:      "",
		imports:  []string{},
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
		filename: "fun_interface.kt",
		pkg:      "com.example",
		imports:  []string{"com.example.other"},
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
		filename: "fun_interface_main.kt",
		pkg:      "com.example",
		imports:  []string{"com.example.other"},
		hasMain:  true,
	},
	{
		desc: "fun-interface-no-imports",
		kt: `
fun interface Runnable {
    fun run()
}
	`,
		filename: "runnable.kt",
		pkg:      "",
		imports:  []string{},
	},
	{
		desc:     "main-simple",
		kt:       "fun main() {}",
		filename: "main.kt",
		imports:  []string{},
		hasMain:  true,
	},
	{
		desc: "main-with-package",
		kt: `
package my.demo
fun main() {}
	`,
		filename: "main_pkg.kt",
		pkg:      "my.demo",
		imports:  []string{},
		hasMain:  true,
	},
	{
		desc: "main-with-args",
		kt: `
fun main(args: Array<String>) {
    println(args)
}
	`,
		filename: "main_args.kt",
		imports:  []string{},
		hasMain:  true,
	},
	{
		desc: "main-with-imports",
		kt: `
package my.demo
import kotlin.text.*
fun main() {}
	`,
		filename: "main_imports.kt",
		pkg:      "my.demo",
		imports:  []string{"kotlin.text"},
		hasMain:  true,
	},
	{
		desc: "suspend-fun-main",
		kt: `
import kotlinx.coroutines.*

suspend fun main() {
    println("hello from coroutines")
}
	`,
		filename: "suspend_main.kt",
		imports:  []string{"kotlinx.coroutines"},
		hasMain:  true,
	},
	{
		desc: "suspend-fun-main-with-args",
		kt: `
suspend fun main(args: Array<String>) {
    delay(1000)
}
	`,
		filename: "suspend_main_args.kt",
		imports:  []string{},
		hasMain:  true,
	},
	{
		desc: "no-main-regular-fun",
		kt: `
fun helper() {}
fun process(x: Int): String { return x.toString() }
	`,
		filename: "helper.kt",
		imports:  []string{},
		hasMain:  false,
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
		filename: "nested_main.kt",
		pkg:      "test.lib",
		imports:  []string{},
		hasMain:  false,
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
		filename: "class_main.kt",
		imports:  []string{},
		hasMain:  false,
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
		filename: "companion_main.kt",
		imports:  []string{},
		hasMain:  false,
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
		filename: "main_after_class.kt",
		imports:  []string{},
		hasMain:  true,
	},
	{
		desc: "single-segment-import-skipped",
		kt: `
import a
import b.C
	`,
		filename: "single_segment.kt",
		imports:  []string{"b"},
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
		filename: "raw_string.kt",
		pkg:      "com.example",
		imports:  []string{"real"},
	},
	{
		desc:     "private-fun-main",
		kt:       "private fun main() {}",
		filename: "private_main.kt",
		imports:  []string{},
		hasMain:  true,
	},
	{
		desc:     "internal-fun-main",
		kt:       "internal fun main() {}",
		filename: "internal_main.kt",
		imports:  []string{},
		hasMain:  true,
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
		filename: "block_comment.kt",
		pkg:      "com.example",
		imports:  []string{"real"},
	},
	{
		desc: "nested-block-comments",
		kt: `
package com.example

/* outer /* inner */ still a comment */

import real.Dep
	`,
		filename: "nested_block.kt",
		pkg:      "com.example",
		imports:  []string{"real"},
	},
	{
		desc: "multiple-modifiers-main",
		kt: `
public suspend fun main() {
    println("hello")
}
	`,
		filename: "multi_mod_main.kt",
		imports:  []string{},
		hasMain:  true,
	},
	{
		desc: "duplicate-package-imports",
		kt: `
import com.example.Foo
import com.example.Bar
	`,
		filename: "dup_imports.kt",
		imports:  []string{"com.example", "com.example"},
	},
}

func TestParser(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			res, errs := Parse(tc.filename, []byte(tc.kt))
			if len(errs) > 0 {
				t.Errorf("Errors parsing %q: %v", tc.filename, errs)
			}

			if res.File != tc.filename {
				t.Errorf("File:\n  actual:   %q\n  expected: %q", res.File, tc.filename)
			}

			if !slices.Equal(res.Imports, tc.imports) {
				t.Errorf("Imports:\n  actual:   %#v\n  expected: %#v\nkotlin code:\n%v", res.Imports, tc.imports, tc.kt)
			}

			if res.Package != tc.pkg {
				t.Errorf("Package:\n  actual:   %#v\n  expected: %#v\nkotlin code:\n%v", res.Package, tc.pkg, tc.kt)
			}

			if res.HasMain != tc.hasMain {
				t.Errorf("HasMain:\n  actual:   %v\n  expected: %v\nkotlin code:\n%v", res.HasMain, tc.hasMain, tc.kt)
			}
		})
	}
}

