package python

import (
	"testing"

	treeutils "github.com/aspect-build/aspect-gazelle/common/treesitter"
)

func TestParsePython(t *testing.T) {
	for _, tc := range []struct {
		desc string
		src  string
	}{
		{
			desc: "empty file",
			src:  "",
		},
		{
			desc: "function definition",
			src: `
def hello():
    return "hello"
`,
		},
		{
			desc: "import statement",
			src:  `import os`,
		},
		{
			desc: "from import statement",
			src:  `from os import path`,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			lang := NewLanguage()
			tree, err := treeutils.ParseSourceCode(lang, "test.py", []byte(tc.src))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tree.Close()
		})
	}
}
