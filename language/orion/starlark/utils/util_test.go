package utils

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.starlark.net/starlark"
)

// iterableOnly is a starlark.Iterable that is NOT a starlark.Sequence
// (it has no Len), so its length is unknown to readers.
type iterableOnly struct {
	items []starlark.Value
}

var _ starlark.Iterable = (*iterableOnly)(nil)

func (it *iterableOnly) String() string        { return "iterableOnly" }
func (it *iterableOnly) Type() string          { return "iterableOnly" }
func (it *iterableOnly) Freeze()               {}
func (it *iterableOnly) Truth() starlark.Bool  { return starlark.True }
func (it *iterableOnly) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: iterableOnly") }
func (it *iterableOnly) Iterate() starlark.Iterator {
	return &iterableOnlyIterator{items: it.items}
}

type iterableOnlyIterator struct {
	items []starlark.Value
	i     int
}

func (it *iterableOnlyIterator) Next(p *starlark.Value) bool {
	if it.i >= len(it.items) {
		return false
	}
	*p = it.items[it.i]
	it.i++
	return true
}

func (it *iterableOnlyIterator) Done() {}

func TestReadWrite(t *testing.T) {
	t.Run("nil <=> None", func(t *testing.T) {
		if Write(nil) != starlark.None {
			t.Errorf("Expected None")
		}

		v, err := Read(starlark.None)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if v != nil {
			t.Errorf("Expected nil")
		}
	})

	t.Run("bool <=> Bool", func(t *testing.T) {
		if Write(true) != starlark.Bool(true) {
			t.Errorf("Expected true")
		}

		v, err := Read(starlark.Bool(true))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if v != true {
			t.Errorf("Expected true")
		}
	})

	t.Run("string <=> String", func(t *testing.T) {
		if Write("hello") != starlark.String("hello") {
			t.Errorf("Expected hello")
		}

		v, err := Read(starlark.String("hello"))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if v != "hello" {
			t.Errorf("Expected hello")
		}
	})

	t.Run("int <=> Int", func(t *testing.T) {
		if Write(123) != starlark.MakeInt(123) {
			t.Errorf("Expected 123")
		}

		v, err := Read(starlark.MakeInt(123))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if v != int64(123) {
			t.Errorf("Expected 123")
		}
	})

	t.Run("float64 <=> Float", func(t *testing.T) {
		if Write(123.45) != starlark.Float(123.45) {
			t.Errorf("Expected 123.45")
		}

		v, err := Read(starlark.Float(123.45))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if v != 123.45 {
			t.Errorf("Expected 123.45")
		}
	})

	t.Run("List => []interface{}", func(t *testing.T) {
		a := ([]any{int64(1), "hello", true})
		l := Write(a).(*starlark.List)

		if len(a) != l.Len() {
			t.Errorf("Expected equal length")
		}

		l0, isInt := l.Index(0).(starlark.Int).Int64()
		if !isInt || a[0] != l0 {
			t.Errorf("Expected %v to be Int64", l0)
		}

		l1, isString := l.Index(1).(starlark.String)
		if !isString || a[1] != l1.GoString() {
			t.Errorf("Expected %v to be String", l1)
		}

		l2, isBool := l.Index(2).(starlark.Bool)
		if !isBool || a[2] != (l2.Truth() == starlark.True) {
			t.Errorf("Expected %v to be Bool", l2)
		}
	})

	t.Run("[]string => List", func(t *testing.T) {
		a := []string{"a", "b"}
		l := Write(a).(*starlark.List)

		if len(a) != l.Len() {
			t.Errorf("Expected equal length")
		}

		l0, isString := l.Index(0).(starlark.String)
		if !isString || a[0] != l0.GoString() {
			t.Errorf("Expected %v to be String", l0)
		}

		l1, isString := l.Index(1).(starlark.String)
		if !isString || a[1] != l1.GoString() {
			t.Errorf("Expected %v to be String", l1)
		}
	})

	t.Run("Iterable without Len => []interface{}", func(t *testing.T) {
		it := &iterableOnly{items: []starlark.Value{
			starlark.MakeInt(1),
			starlark.String("hello"),
			starlark.Bool(true),
		}}

		av, err := Read(it)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		a, isSlice := av.([]any)
		if !isSlice {
			t.Fatalf("Expected []any, got %T", av)
		}
		if len(a) != 3 {
			t.Fatalf("Expected 3 elements, got %d", len(a))
		}
		if a[0] != int64(1) {
			t.Errorf("Expected 1, got %v", a[0])
		}
		if a[1] != "hello" {
			t.Errorf("Expected hello, got %v", a[1])
		}
		if a[2] != true {
			t.Errorf("Expected true, got %v", a[2])
		}
	})

	t.Run("List <=> []interface{}", func(t *testing.T) {
		l := starlark.NewList([]starlark.Value{starlark.MakeInt(1), starlark.String("hello"), starlark.Bool(true)})
		av, err := Read(l)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		a := av.([]any)
		if len(a) != l.Len() {
			t.Errorf("Expected equal length")
		}

		l0, isInt := l.Index(0).(starlark.Int).Int64()
		if !isInt || a[0].(int64) != l0 {
			t.Errorf("Expected %v to be Int64", l0)
		}

		l1, isString := l.Index(1).(starlark.String)
		if !isString || a[1] != l1.GoString() {
			t.Errorf("Expected %v to be String", l1)
		}

		l2, isBool := l.Index(2).(starlark.Bool)
		if !isBool || a[2] != (l2.Truth() == starlark.True) {
			t.Errorf("Expected %v to be Bool", l2)
		}
	})
}

func TestErrorStr(t *testing.T) {
	t.Run("plain error includes pre context", func(t *testing.T) {
		s := ErrorStr("Failed to invoke myplugin:Prepare()", errors.New("boom"))
		if !strings.Contains(s, "Failed to invoke myplugin:Prepare()") {
			t.Errorf("Expected pre context in %q", s)
		}
		if !strings.Contains(s, "boom") {
			t.Errorf("Expected error message in %q", s)
		}
	})

	t.Run("EvalError includes pre context and backtrace", func(t *testing.T) {
		_, err := starlark.ExecFile(&starlark.Thread{}, "test.star", "fail(\"boom\")", nil)
		if err == nil {
			t.Fatal("Expected an error")
		}
		if _, isEvalError := err.(*starlark.EvalError); !isEvalError {
			t.Fatalf("Expected *starlark.EvalError, got %T", err)
		}

		s := ErrorStr("Failed to invoke myplugin:Analyze()", err)
		if !strings.Contains(s, "Failed to invoke myplugin:Analyze()") {
			t.Errorf("Expected pre context in %q", s)
		}
		if !strings.Contains(s, "boom") {
			t.Errorf("Expected error message in %q", s)
		}
		if !strings.Contains(s, "Traceback") {
			t.Errorf("Expected backtrace in %q", s)
		}
	})

	t.Run("empty pre returns error message only", func(t *testing.T) {
		if s := ErrorStr("", errors.New("boom")); s != "boom" {
			t.Errorf("Expected %q, got %q", "boom", s)
		}
	})
}
