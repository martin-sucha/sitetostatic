// Package repository implements storing HTTP responses in filesystem.
//
// Files are stored in a directory with the cache key in filename encoded using base32.
// Base32 is used so that the encoding will work on case insensitive filesystems.
//
// File format of individual files is as follows:
//
//  Field        Type             Description
//  magic        [4]byte          "STS1" identifying the file format
//  body_size    uint64_le        length of body data in bytes
//  body_sha256  [32]byte         SHA-256 digest of body data
//  json_size    uint32_le        length of JSON data in bytes
//  json_crc32   uint32_le        IEEE crc32 checksum of JSON data
//  body_data    [body_size]byte  Data of the body
//  json_data    [json_size]byte  JSON data describing the request
package repository

import (
	"bytes"
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

type DocumentMetadata struct {
	Key                 string
	DownloadStartedTime time.Time
	URL                 string
	Headers             http.Header
}

type Document struct {
	Metadata   DocumentMetadata
	BodySHA256 [sha256.Size]byte
	BodySize   int64
	f          *os.File
	bodyOffset int64
}

func New(path string) *Repository {
	return &Repository{path: path}
}

const binaryHeaderSize = 52

func (r *Repository) Store(h *DocumentMetadata, bodyData io.Reader) (outErr error) {
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

	_, err = f.Seek(binaryHeaderSize, 0)
	if err != nil {
		return err
	}

	bodyHasher := sha256.New()
	bodyWrittenBytes, err := io.Copy(f, io.TeeReader(bodyData, bodyHasher))
	if err != nil {
		return err
	}

	jsonData, err := json.Marshal(h)
	if err != nil {
		return
	}
	if len(jsonData) > math.MaxUint32 {
		return fmt.Errorf("json data size overflow: %d bytes", len(jsonData))
	}

	_, err = f.Write(jsonData)
	if err != nil {
		return err
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		return err
	}

	var binaryHeader [binaryHeaderSize]byte
	copy(binaryHeader[0:4], "STS1")
	binary.LittleEndian.PutUint64(binaryHeader[4:12], uint64(bodyWrittenBytes))
	bodyHasher.Sum(binaryHeader[12:12:44])
	binary.LittleEndian.PutUint32(binaryHeader[44:48], uint32(len(jsonData)))
	binary.LittleEndian.PutUint32(binaryHeader[48:52], crc32.ChecksumIEEE(jsonData))

	_, err = f.Write(binaryHeader[:])
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	closed = true
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
	if !bytes.Equal(binaryHeader[0:4], []byte("STS1")) {
		return nil, fmt.Errorf("incorrect magic")
	}

	doc := &Document{
		f: f,
	}
	doc.BodySize = int64(binary.LittleEndian.Uint64(binaryHeader[4:12]))
	copy(doc.BodySHA256[:], binaryHeader[12:44])

	jsonDataSize := binary.LittleEndian.Uint32(binaryHeader[44:48])
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

func keyToFilename(key string) string {
	encodedSize := base32.StdEncoding.EncodedLen(len(key))
	buf := make([]byte, encodedSize+4)
	base32.StdEncoding.Encode(buf, []byte(key))
	copy(buf[encodedSize:], ".bin")
	return string(buf)
}
