package cache

import (
	"path"
)

var _ Cache = (*noopCache)(nil)

var noop Cache = &noopCache{}

type noopCache struct{}

func (c *noopCache) LoadOrStoreFile(root, p, key string, loader FileCompute) (any, bool, error) {
	return withFileContent(path.Join(root, p), func(content []byte) (any, bool, error) {
		result, err := loader(p, content)
		return result, false, err
	})
}

func (c *noopCache) Persist() {}
