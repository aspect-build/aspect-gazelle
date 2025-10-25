package starlark

import (
	"testing"

	"go.starlark.net/starlark"
)

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
		a := ([]interface{}{int64(1), "hello", true})
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

	t.Run("List <=> []interface{}", func(t *testing.T) {
		l := starlark.NewList([]starlark.Value{starlark.MakeInt(1), starlark.String("hello"), starlark.Bool(true)})
		av, err := Read(l)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		a := av.([]interface{})
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
