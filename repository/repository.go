// Package repository implements storing HTTP responses in filesystem.
//
// Files are stored in a directory with the cache key in filename encoded using base32.
// Base32 is used so that the encoding will work on case insensitive filesystems.
//
// File format of individual files is as follows:
//
//	Field        Type             Description
//	magic        [4]byte          "STS1" identifying the file format
//	body_size    uint64_le        length of body data in bytes
//	body_sha256  [32]byte         SHA-256 digest of body data
//	json_size    uint32_le        length of JSON data in bytes
//	json_crc32   uint32_le        IEEE crc32 checksum of JSON data
//	body_data    [body_size]byte  Data of the body
//	json_data    [json_size]byte  JSON data describing the request
package repository

import (
	"bytes"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

type Repository struct {
	path string
}

type DocumentMetadata struct {
	Key                 string
	DownloadStartedTime time.Time
	URL                 string
	Status              string
	StatusCode          int
	Proto               string
	Headers             http.Header
	Trailers            http.Header
}

type Document struct {
	Metadata   DocumentMetadata
	BodySHA256 [sha256.Size]byte
	BodySize   int64
	f          *os.File
}

func (d *Document) Body() *io.SectionReader {
	return io.NewSectionReader(d.f, binaryHeaderSize, d.BodySize)
}

func (d *Document) Close() error {
	return d.f.Close()
}

func New(path string) *Repository {
	return &Repository{path: path}
}

const binaryHeaderSize = 52

func (r *Repository) NewWriter() (dwOut *DocumentWriter, outErr error) {
	f, err := ioutil.TempFile(r.path, "tmp-")
	if err != nil {
		return nil, err
	}
	defer func() {
		if outErr != nil {
			// TODO: log errors
			_ = f.Close()
			_ = os.Remove(f.Name())
		}
	}()

	_, err = f.Seek(binaryHeaderSize, 0)
	if err != nil {
		return nil, err
	}

	dw := &DocumentWriter{
		r:          r,
		f:          f,
		bodyHasher: sha256.New(),
	}
	return dw, nil
}

type DocumentWriter struct {
	r                *Repository
	f                *os.File
	bodyHasher       hash.Hash
	bodyWrittenBytes uint64
}

func (d *DocumentWriter) Write(b []byte) (n int, err error) {
	_, err2 := d.bodyHasher.Write(b)
	if err2 != nil {
		return 0, err2
	}
	n, err = d.f.Write(b)
	d.bodyWrittenBytes += uint64(n)
	return
}

func (d *DocumentWriter) Close(metadata *DocumentMetadata) error {
	closed := false
	defer func() {
		if !closed {
			// TODO: log errors
			_ = d.f.Close()
			_ = os.Remove(d.f.Name())
		}
	}()

	jsonData, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	if len(jsonData) > math.MaxUint32 {
		return fmt.Errorf("json data size overflow: %d bytes", len(jsonData))
	}

	_, err = d.f.Write(jsonData)
	if err != nil {
		return err
	}

	_, err = d.f.Seek(0, 0)
	if err != nil {
		return err
	}

	var binaryHeader [binaryHeaderSize]byte
	copy(binaryHeader[0:4], "STS1")
	binary.LittleEndian.PutUint64(binaryHeader[4:12], d.bodyWrittenBytes)
	d.bodyHasher.Sum(binaryHeader[12:12:44])
	binary.LittleEndian.PutUint32(binaryHeader[44:48], uint32(len(jsonData)))
	binary.LittleEndian.PutUint32(binaryHeader[48:52], crc32.ChecksumIEEE(jsonData))

	_, err = d.f.Write(binaryHeader[:])
	if err != nil {
		return err
	}
	err = d.f.Close()
	closed = true
	if err != nil {
		return err
	}
	filename := keyToFilename(metadata.Key)
	return os.Rename(d.f.Name(), path.Join(d.r.path, filename))
}

func (r *Repository) Load(key string) (outDoc *Document, outErr error) {
	return openDocumentPath(path.Join(r.path, keyToFilename(key)))
}

func openDocumentPath(filePath string) (outDoc *Document, outErr error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if outErr != nil {
			// TODO: log error.
			_ = f.Close()
		}
	}()
	doc, err := openDocument(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", filePath, err)
	}
	return doc, nil
}

func openDocument(f *os.File) (*Document, error) {
	var binaryHeader [binaryHeaderSize]byte
	_, err := io.ReadFull(f, binaryHeader[:])
	switch {
	case errors.Is(err, io.EOF):
		return nil, io.ErrUnexpectedEOF
	case err != nil:
		return nil, err
	}
	if !bytes.Equal(binaryHeader[0:4], []byte("STS1")) {
		return nil, fmt.Errorf("incorrect magic")
	}

	doc := &Document{
		f: f,
	}
	doc.BodySize = int64(binary.LittleEndian.Uint64(binaryHeader[4:12]))
	copy(doc.BodySHA256[:], binaryHeader[12:44])

	jsonDataSize := binary.LittleEndian.Uint32(binaryHeader[44:48])

	_, err = f.Seek(binaryHeaderSize+doc.BodySize, 0)
	if err != nil {
		return nil, err
	}

	jsonData := make([]byte, jsonDataSize)
	jsonChecksum := crc32.NewIEEE()
	_, err = io.ReadFull(io.TeeReader(f, jsonChecksum), jsonData)
	switch {
	case errors.Is(err, io.EOF):
		return nil, io.ErrUnexpectedEOF
	case err != nil:
		return nil, err
	}
	jsonExpectedChecksum := binary.LittleEndian.Uint32(binaryHeader[48:52])
	if jsonChecksum.Sum32() != jsonExpectedChecksum {
		return nil, fmt.Errorf("crc32 checksum of metadata json does not match")
	}

	err = json.Unmarshal(jsonData, &doc.Metadata)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

type Entry struct {
	r        *Repository
	filename string
}

func (e *Entry) Open() (*Document, error) {
	return openDocumentPath(path.Join(e.r.path, e.filename))
}

func (r *Repository) List() ([]Entry, error) {
	f, err := os.Open(r.path)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	closeErr := f.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	entries := make([]Entry, 0, len(names))
	for _, name := range names {
		if strings.HasPrefix(name, "tmp-") {
			continue
		}
		if !strings.HasSuffix(name, ".bin") {
			continue
		}
		entries = append(entries, Entry{
			r:        r,
			filename: name,
		})
	}
	return entries, nil
}

func keyToFilename(key string) string {
	encodedSize := base32.StdEncoding.EncodedLen(len(key))
	buf := make([]byte, encodedSize+4)
	base32.StdEncoding.Encode(buf, []byte(key))
	copy(buf[encodedSize:], ".bin")
	return string(buf)
}
