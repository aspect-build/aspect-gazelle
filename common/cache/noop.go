package cache

import (
	"path"
)

var _ Cache = (*noopCache)(nil)

var noop Cache = &noopCache{}

type noopCache struct{}

func (c *noopCache) LoadOrStoreFile(root, p, key string, loader FileCompute) (any, bool, error) {
	content, release, err := readFile(path.Join(root, p))
	if err != nil {
		return nil, false, err
	}
	defer release()

	result, err := loader(p, content)
	return result, false, err
}

func (c *noopCache) Persist() {}
