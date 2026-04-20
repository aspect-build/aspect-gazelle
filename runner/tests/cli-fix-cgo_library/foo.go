package cgo_test

// #include <stdio.h>
import "C"

func Hello() {
	C.puts(C.CString("hello"))
}
