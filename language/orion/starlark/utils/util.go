package starlark

import (
	"fmt"
	"log"

	"go.starlark.net/starlark"
)

var EmptyKwArgs = make([]starlark.Tuple, 0)
var EmptyStrings = make([]string, 0)

func Write(v interface{}) starlark.Value {
	if sv, isSV := v.(starlark.Value); isSV {
		return sv
	}

	// Primitive types
	switch v := v.(type) {
	case nil:
		return starlark.None
	case bool:
		return starlark.Bool(v)
	case string:
		return starlark.String(v)
	case int:
		return starlark.MakeInt(v)
	case int64:
		return starlark.MakeInt64(v)
	case float64:
		return starlark.Float(v)
	case []string:
		return WriteList(v, WriteString)
	case []interface{}:
		return WriteList(v, Write)
	case map[string]interface{}:
		return WriteMap(v, Write)
	}

	log.Panicf("Failed to write value %v of type %T", v, v)
	return nil
}

func WriteString(v string) starlark.Value {
	return starlark.String(v)
}

func WriteList[V any](a []V, f func(v V) starlark.Value) starlark.Value {
	l := make([]starlark.Value, 0, len(a))
	for _, v := range a {
		l = append(l, f(v))
	}
	return starlark.NewList(l)
}

func WriteMap[K any](m map[string]K, f func(v K) starlark.Value) starlark.Value {
	d := starlark.NewDict(len(m))
	for k, v := range m {
		d.SetKey(starlark.String(k), f(v))
	}
	return d
}

func WriteStringMap(m map[string]string) starlark.Value {
	return WriteMap(m, WriteString)
}

func ReadBool(v starlark.Value) (bool, error) {
	bo, ok := v.(starlark.Bool)
	if !ok {
		return false, fmt.Errorf("expected bool, got %T", v)
	}
	return bo.Truth() == starlark.True, nil
}

func ReadString(v starlark.Value) (string, error) {
	s, ok := v.(starlark.String)
	if !ok {
		return "", fmt.Errorf("expected string, got %T", v)
	}
	return s.GoString(), nil
}

func ReadList[V any](v starlark.Value, f func(v starlark.Value) (V, error)) ([]V, error) {
	l, isList := v.(*starlark.List)
	if !isList {
		return nil, fmt.Errorf("expected list, got %T", v)
	}
	len := l.Len()
	a := make([]V, 0, len)
	for i := range len {
		v, err := f(l.Index(i))
		if err != nil {
			return nil, err
		}
		a = append(a, v)
	}
	return a, nil
}

func ReadTuple[V any](t starlark.Tuple, f func(v starlark.Value) (V, error)) ([]V, error) {
	len := t.Len()
	a := make([]V, 0, len)
	for i := range len {
		v, err := f(t.Index(i))
		if err != nil {
			return nil, err
		}
		a = append(a, v)
	}
	return a, nil
}

func ReadStringList(l starlark.Value) ([]string, error) {
	return ReadList(l, ReadString)
}

func ReadStringTuple(l starlark.Tuple) ([]string, error) {
	return ReadTuple(l, ReadString)
}

func Read(v starlark.Value) (interface{}, error) {
	return ReadRecurse(v, Read)
}

func ReadRecurse(v starlark.Value, read func(v starlark.Value) (interface{}, error)) (interface{}, error) {
	switch v := v.(type) {
	case starlark.NoneType:
		return nil, nil
	case starlark.Bool:
		return v.Truth() == starlark.True, nil
	case starlark.String:
		return v.GoString(), nil
	case starlark.Int:
		i, _ := v.Int64()
		return i, nil
	case starlark.Float:
		return float64(v), nil
	case *starlark.List:
		return ReadList(v, read)
	case *starlark.Dict:
		return ReadMap2(v, read)
	case starlark.Sequence:
		return readIterable(v, v.Len(), read)
	case starlark.Iterable:
		return readIterable(v, -1, read)
	case starlark.Indexable:
		return readIndexable(v, read)
	}

	return nil, fmt.Errorf("failed to read starlark value %T", v)
}

func readIterable(v starlark.Iterable, len int, read func(v starlark.Value) (interface{}, error)) (interface{}, error) {
	iter := v.Iterate()
	defer iter.Done()

	a := make([]interface{}, 0, len)
	var x starlark.Value
	for iter.Next(&x) {
		val, err := read(x)
		if err != nil {
			return nil, err
		}
		a = append(a, val)
	}

	return a, nil
}

func readIndexable(v starlark.Indexable, read func(v starlark.Value) (interface{}, error)) ([]interface{}, error) {
	len := v.Len()
	a := make([]interface{}, 0, len)
	for i := range len {
		val, err := read(v.Index(i))
		if err != nil {
			return nil, err
		}
		a = append(a, val)
	}
	return a, nil
}

func ReadMap[K any](v starlark.Value, f func(k string, v starlark.Value) (K, error)) (map[string]K, error) {
	d := v.(*starlark.Dict)
	m := make(map[string]K, d.Len())

	iter := d.Iterate()
	defer iter.Done()

	var kv starlark.Value
	for iter.Next(&kv) {
		k, err := ReadString(kv)
		if err != nil {
			return nil, err
		}
		v, _, _ := d.Get(kv)
		mv, err := f(k, v)
		if err != nil {
			return nil, err
		}
		m[k] = mv
	}

	return m, nil
}

func ReadMap2[K any](v starlark.Value, f func(v starlark.Value) (K, error)) (map[string]K, error) {
	d := v.(*starlark.Dict)
	m := make(map[string]K, d.Len())

	iter := d.Iterate()
	defer iter.Done()

	var kv starlark.Value
	for iter.Next(&kv) {
		k, err := ReadString(kv)
		if err != nil {
			return nil, err
		}
		v, _, _ := d.Get(kv)
		mv, err := f(v)
		if err != nil {
			return nil, err
		}
		m[k] = mv
	}

	return m, nil
}

func ReadMapEntry[K any](v starlark.Value, key string, f func(v starlark.Value) (K, error), defaultValue K) (K, error) {
	m := v.(*starlark.Dict)
	val, exists, err := (*m).Get(starlark.String(key))

	if err != nil {
		return defaultValue, fmt.Errorf("failed to read map entry '%s': %v", key, err)
	}

	if !exists {
		return defaultValue, nil
	}

	return f(val)
}
