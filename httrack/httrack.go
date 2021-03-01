// Package httrack implements reading of httrack cache.
//
// See https://www.httrack.com/html/cache.html
//
// You can use -k httrack option to store all content in the cache.
package httrack

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"site-to-static/httrack/internal/zip"
	"strconv"
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

// FindEntry returns the first Entry for which fn returns true.
// Returns nil if fn returns false for all entries.
func (c *Cache) FindEntry(fn func(e *Entry) bool) *Entry {
	for i := range c.Entries {
		if fn(c.Entries[i]) {
			return c.Entries[i]
		}
	}
	return nil
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
		// Add empty line to the end to prevent http.ReadResponse from returning io.UnexpectedEOF.
		var buf bytes.Buffer
		buf.Grow(len(extra) + 2)
		buf.Write(extra)
		buf.WriteString("\n\n")
		rq, err := http.ReadResponse(bufio.NewReaderSize(&buf, buf.Len()), nil)
		if err != nil {
			return nil, err
		}
		_ = rq.Body.Close()
		header := rq.Header
		entry := &Entry{
			zf:         f,
			URL:        f.Name,
			Extra:      string(extra),
			Proto:      rq.Proto,
			StatusCode: rq.StatusCode,
			Status:     rq.Status,
			Header:     header,
		}

		if size := header.Get("x-size"); size != "" {
			parsedSize, err := strconv.ParseInt(size, 10, 64)
			if err == nil {
				entry.Size = parsedSize
				header.Del("x-size")
			}
		}
		header.Del("x-statuscode")
		header.Del("x-statusmessage")
		if inCache := header.Get("x-in-cache"); inCache != "" {
			switch inCache {
			case "1":
				entry.InCache = true
			case "0":
				entry.InCache = false
			default:
				return nil, fmt.Errorf("unrecognized value for X-In-Cache: %q", inCache)
			}
			header.Del("x-in-cache")
		}
		cache.Entries = append(cache.Entries, entry)
	}
	return cache, nil
}

type Entry struct {
	// URL of the downloaded resource.
	URL string
	// Status line from HTTP protocol.
	Status string
	// StatusCode of the response.
	StatusCode int
	// Proto is version of HTTP protocol (e.g. HTTP/1.1)
	Proto string
	// Header is a map of headers plus extra data stored by httrack.
	Header http.Header
	// InCache indicates whether the resource was stored in the cache or not.
	InCache bool
	// Size of the content.
	Size int64
	// Extra is raw string of the metadata stored by httrack.
	Extra string
	// zf is zip file representing the resource.
	zf *zip.File
}

func (e *Entry) Body() (io.ReadCloser, error) {
	return e.zf.Open()
}
