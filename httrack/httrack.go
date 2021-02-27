// Package httrack implements reading of httrack cache.
//
// See https://www.httrack.com/html/cache.html
package httrack

import (
	"io"
	"site-to-static/httrack/internal/zip"
)

type Cache struct {
	Entries []*Entry
	z       *zip.Reader
	closer  io.Closer
}

func OpenCache(name string) (cache *Cache, errOut error) {
	z, err := zip.OpenReader(name)
	if err != nil {
		return nil, err
	}
	defer func() {
		if errOut != nil {
			_ = z.Close()
		}
	}()
	return loadCache(&z.Reader, z)
}

func NewCache(r io.ReaderAt, size int64) (*Cache, error) {
	z, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	return loadCache(z, nil)
}

func (c *Cache) Close() error {
	if c.closer == nil {
		return nil
	}
	return c.closer.Close()
}

func loadCache(z *zip.Reader, closer io.Closer) (*Cache, error) {
	cache := &Cache{
		z:       z,
		closer:  closer,
		Entries: make([]*Entry, 0, len(z.File)),
	}
	for _, f := range z.File {
		extra, err := f.LocalExtraField()
		if err != nil {
			return nil, err
		}
		cache.Entries = append(cache.Entries, &Entry{
			URL:   f.Name,
			Extra: string(extra),
		})
	}
	return cache, nil
}

type Entry struct {
	URL   string
	Extra string
}
