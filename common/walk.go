package gazelle

import (
	"log"
	"slices"
	"strings"
	"sync"

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
	if len(d.Subdirs) == 0 {
		return d.RegularFiles, nil
	}

	// Use channels to collect results from parallel goroutines.
	// Gazelle may populate the initial directory cache of the initially walked directories
	// but incremental runs will not have walked lazy-indexed subdirectories.
	resultChan := make(chan string, len(d.Subdirs))
	wg := &sync.WaitGroup{}

	if rel != "" {
		rel = rel + "/"
	}
	collectSourceRegularSubFiles(wg, rel, "", d, resultChan)

	// Close the channel when all goroutines are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results from all goroutines
	files := d.RegularFiles[:]
	for res := range resultChan {
		files = append(files, res)
	}
	slices.Sort(files)
	return files, nil
}

func collectSourceRegularSubFiles(wg *sync.WaitGroup, base, rel string, d walk.DirInfo, resultChan chan<- string) {
	for _, sdRel := range d.Subdirs {
		wg.Add(1)
		go func(sdRel string) {
			defer wg.Done()

			if rel != "" {
				sdRel = rel + "/" + sdRel
			}

			sdInfo, _ := walk.GetDirInfo(base + sdRel)

			// Recurse into subdirectories that do not have a BUILD file just like a
			// bazel BUILD glob() would.
			if sdInfo.File == nil {
				for _, f := range sdInfo.RegularFiles {
					resultChan <- sdRel + "/" + f
				}

				collectSourceRegularSubFiles(wg, base, sdRel, sdInfo, resultChan)
			}
		}(sdRel)
	}
}
