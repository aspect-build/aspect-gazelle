package cache

import (
	"io"
	"os"
	"sync"
)

// bufPool recycles file-read buffers, turning per-file read allocation (the
// dominant heap churn) into a small set of reused buffers. We store *[]byte, not
// []byte, to avoid boxing the slice header on every Put.
var bufPool = sync.Pool{New: func() any { return new([]byte) }}

// maxPooledBuffer caps pooled capacity: buffers grown past this by a large file
// are dropped on release rather than pinned for later small reads.
const maxPooledBuffer = 1 << 20 // 1 MiB

// readFile reads the file into a pooled buffer, returning its contents and a
// release func the caller MUST call once done. The bytes are only valid until
// release(); callers must copy out anything they retain (the parsers here do).
func readFile(name string) (content []byte, release func(), err error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	size := int(fi.Size())

	bufp := bufPool.Get().(*[]byte)
	buf := *bufp
	if cap(buf) < size {
		buf = make([]byte, size)
	} else {
		buf = buf[:size]
	}

	// Return to the pool unless it grew too large to keep.
	release = func() {
		if cap(buf) <= maxPooledBuffer {
			*bufp = buf
			bufPool.Put(bufp)
		}
	}

	if _, err := io.ReadFull(f, buf); err != nil {
		release()
		return nil, nil, err
	}

	return buf, release, nil
}
