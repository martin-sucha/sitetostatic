package repository

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path"
	"time"
)

type Repository struct {
	path string
}

type DocumentHeader struct {
	Key            string
	DownloadedDate time.Time
	URL            string
	Headers        http.Header
}

type Document struct {
	Header     DocumentHeader
	f          *os.File
	bodyOffset int64
}

func New(path string) *Repository {
	return &Repository{path: path}
}

const binaryHeaderSize = 8 + sha256.Size

func (r *Repository) Store(h *DocumentHeader, data io.Reader) (outErr error) {
	filename := keyToFilename(h.Key)
	f, err := ioutil.TempFile(r.path, "tmp-"+filename)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		var closeErr error
		if !closed {
			closeErr = f.Close()
		}
		if outErr == nil {
			outErr = closeErr
		} else {
			// TODO: log error
			_ = os.Remove(f.Name())
		}
	}()
	headerData, err := json.Marshal(h)
	if err != nil {
		return
	}
	if len(headerData) > math.MaxUint32 {
		return fmt.Errorf("json header size overflow: %d bytes", len(headerData))
	}

	var binaryHeader [binaryHeaderSize]byte
	binary.LittleEndian.PutUint32(binaryHeader[0:4], uint32(len(headerData)))
	binary.LittleEndian.PutUint32(binaryHeader[4:8], crc32.ChecksumIEEE(headerData))
	_, err = f.Write(binaryHeader[:])
	if err != nil {
		return err
	}
	_, err = f.Write(headerData)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	_, err = io.Copy(f, io.TeeReader(data, hasher))
	if err != nil {
		return err
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}
	hasher.Sum(binaryHeader[8:8:binaryHeaderSize])
	_, err = f.Write(binaryHeader[:])
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path.Join(r.path, filename))
}

func (r *Repository) Load(key string) (outDoc *Document, outErr error) {
	f, err := os.Open(path.Join(r.path, keyToFilename(key)))
	if err != nil {
		return nil, err
	}
	defer func() {
		if outErr != nil {
			// TODO: log error.
			_ = f.Close()
		}
	}()
	var binaryHeader [binaryHeaderSize]byte
	_, err = io.ReadFull(f, binaryHeader[:])
	switch {
	case errors.Is(err, io.EOF):
		return nil, io.ErrUnexpectedEOF
	case err != nil:
		return nil, err
	}
	jsonHeaderSize := binary.LittleEndian.Uint32(binaryHeader[0:4])
	jsonData := make([]byte, jsonHeaderSize)
	_, err = io.ReadFull(f, jsonData)
	switch {
	case errors.Is(err, io.EOF):
		return nil, io.ErrUnexpectedEOF
	case err != nil:
		return nil, err
	}
	doc := &Document{
		f:          f,
		bodyOffset: binaryHeaderSize + int64(jsonHeaderSize),
	}
	err = json.Unmarshal(jsonData, &doc.Header)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func keyToFilename(key string) string {
	encodedSize := base32.StdEncoding.EncodedLen(len(key))
	buf := make([]byte, encodedSize+4)
	base32.StdEncoding.Encode(buf, []byte(key))
	copy(buf[encodedSize:], ".bin")
	return string(buf)
}
