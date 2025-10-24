package gazelle

import (
	"log"
	"slices"
	"strings"

	"github.com/bazelbuild/bazel-gazelle/walk"
)

func WalkHasPath(rel, p string) bool {
	d, err := walk.GetDirInfo(rel)
	if err != nil {
		log.Fatal(err)
	}

	// Navigate into subdirectories...
	// - Do not allocate arrays such as string.Split()
	// - Do not allow path.Join/Clean() multiple times on the same paths
	for i := strings.IndexByte(p, '/'); i >= 0; i = strings.IndexByte(p, '/') {
		subdir := p[:i]
		if !slices.Contains(d.Subdirs, subdir) {
			return false
		}
		if rel == "" {
			rel = subdir
		} else {
			rel = rel + "/" + subdir
		}
		d, err = walk.GetDirInfo(rel)
		if err != nil {
			log.Fatal(err)
		}

		p = p[i+1:]
	}

	return slices.Contains(d.RegularFiles, p)
}

func GetSourceRegularFiles(rel string) ([]string, error) {
	d, err := walk.GetDirInfo(rel)
	if err != nil {
		return nil, err
	}

	if rel != "" {
		rel = rel + "/"
	}
	return getSourceRegularSubFiles(rel, "", d, d.RegularFiles[:])
}

func getSourceRegularSubFiles(base, rel string, d walk.DirInfo, files []string) ([]string, error) {
	for _, sdRel := range d.Subdirs {
		if rel != "" {
			sdRel = rel + "/" + sdRel
		}

		sdInfo, err := walk.GetDirInfo(base + sdRel)
		if err != nil {
			continue
		}

		// Recurse into subdirectories that do not have a BUILD file just like a
		// bazel BUILD glob() would.
		if sdInfo.File == nil {
			for _, f := range sdInfo.RegularFiles {
				files = append(files, sdRel+"/"+f)
			}

			files, _ = getSourceRegularSubFiles(base, sdRel, sdInfo, files)
		}
	}

	return files, nil
}
