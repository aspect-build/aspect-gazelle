package rule

import (
	"reflect"
	"testing"

	bzl "github.com/bazelbuild/buildtools/build"
)

func parseExpr(t *testing.T, expr string) bzl.Expr {
	t.Helper()

	file, err := bzl.Parse("test.bzl", []byte("x = "+expr+"\n"))
	if err != nil {
		t.Fatalf("parse expr: %v", err)
	}
	if len(file.Stmt) != 1 {
		t.Fatalf("parse expr: expected 1 statement, got %d", len(file.Stmt))
	}
	assign, ok := file.Stmt[0].(*bzl.AssignExpr)
	if !ok {
		t.Fatalf("parse expr: expected assignment, got %T", file.Stmt[0])
	}
	return assign.RHS
}

func TestExpandSrcsConcatenatedGlob(t *testing.T) {
	t.Run("empty_list_plus_glob", func(t *testing.T) {
		files := []string{"a.go", "b.go", "c.txt", "dir/d.go"}
		expr := parseExpr(t, "[] + glob([\"*.go\"])")

		got, err := ExpandSrcs(files, expr)
		if err != nil {
			t.Fatalf("ExpandSrcs: %v", err)
		}
		want := []string{"a.go", "b.go"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExpandSrcs: got %v, want %v", got, want)
		}
	})

	t.Run("glob_plus_list_plus_glob", func(t *testing.T) {
		files := []string{"a.go", "b.go", "c.txt", "dir/d.go"}
		expr := parseExpr(t, "glob([\"*.go\"]) + [\"manual.go\"] + glob([\"dir/*.go\"])")

		got, err := ExpandSrcs(files, expr)
		if err != nil {
			t.Fatalf("ExpandSrcs: %v", err)
		}
		want := []string{"a.go", "b.go", "manual.go", "dir/d.go"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExpandSrcs: got %v, want %v", got, want)
		}
	})
}

func TestExpandSrcsListAndGlob(t *testing.T) {
	t.Run("basic_list", func(t *testing.T) {
		files := []string{"a.go", "b.go"}
		expr := parseExpr(t, "[\"a.go\", \"b.go\"]")

		got, err := ExpandSrcs(files, expr)
		if err != nil {
			t.Fatalf("ExpandSrcs: %v", err)
		}
		want := []string{"a.go", "b.go"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExpandSrcs: got %v, want %v", got, want)
		}
	})

	t.Run("concat_lists", func(t *testing.T) {
		files := []string{"a.go", "b.go"}
		expr := parseExpr(t, "[\"a.go\"] + [\"b.go\"]")

		got, err := ExpandSrcs(files, expr)
		if err != nil {
			t.Fatalf("ExpandSrcs: %v", err)
		}
		want := []string{"a.go", "b.go"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExpandSrcs: got %v, want %v", got, want)
		}
	})

	t.Run("concat_list_and_glob", func(t *testing.T) {
		files := []string{"a.go", "dir/d.go", "dir/e.txt"}
		expr := parseExpr(t, "[\"manual.go\"] + glob([\"dir/*.go\"])")

		got, err := ExpandSrcs(files, expr)
		if err != nil {
			t.Fatalf("ExpandSrcs: %v", err)
		}
		want := []string{"manual.go", "dir/d.go"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExpandSrcs: got %v, want %v", got, want)
		}
	})

	t.Run("glob_with_excludes", func(t *testing.T) {
		files := []string{"a.go", "b.go", "c.txt", "dir/d.go"}
		expr := parseExpr(t, "glob([\"*.go\"], exclude=[\"b.go\"])")

		got, err := ExpandSrcs(files, expr)
		if err != nil {
			t.Fatalf("ExpandSrcs: %v", err)
		}
		want := []string{"a.go"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ExpandSrcs: got %v, want %v", got, want)
		}
	})
}
