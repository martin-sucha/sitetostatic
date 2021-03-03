// Package files is a target to generate output files.
// See https://serverfault.com/a/276755 when you have URLs with query string.
package files

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/martin-sucha/site-to-static/repository"
	"github.com/martin-sucha/site-to-static/urlnorm"
)

func Generate(repo *repository.Repository, outDir string) error {
	entries, err := repo.List()
	if err != nil {
		return err
	}
	err = os.Mkdir(outDir, 0777)
	if err != nil {
		return err
	}
	for _, e := range entries {
		doc, err := e.Open()
		if err != nil {
			return err
		}
		err = processEntry(doc, outDir)
		closeErr := doc.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func processEntry(doc *repository.Document, outDir string) error {
	u, err := url.Parse(doc.Metadata.URL)
	if err != nil {
		return err
	}
	uc := urlnorm.Canonical(u)
	switch {
	case doc.Metadata.StatusCode == 404:
		// skip
		return nil
	case doc.Metadata.StatusCode == 200:
		dir := fmt.Sprintf("%s-%s-%s", uc.Scheme, uc.Hostname(), resolvePort(uc.Scheme, uc.Port()))
		err := os.MkdirAll(filepath.Join(outDir, dir), 0777)
		if err != nil {
			return err
		}
		mediatype, _, err := mime.ParseMediaType(doc.Metadata.Headers.Get("content-type"))
		if err != nil {
			return err
		}
		filename := u.Path
		if u.RawQuery != "" {
			filename += "?" + u.RawQuery
		} else if strings.HasSuffix(u.Path, "/") {
			filename += "index"
		}
		if mediatype == "text/html" && !htmlExtensionRe.MatchString(filename) {
			filename += ".html"
		}
		outputPath := filepath.Join(outDir, dir, filename)
		dir, _ = filepath.Split(outputPath)
		err = os.MkdirAll(dir, 0777)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, doc.Body())
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		mtime := doc.Metadata.DownloadStartedTime
		if lastModified := doc.Metadata.Headers.Get("Last-Modified"); lastModified != "" {
			parsedTime, err := http.ParseTime(lastModified)
			if err != nil {
				return err
			}
			mtime = parsedTime
		}
		return os.Chtimes(outputPath, mtime, mtime)
	case 300 <= doc.Metadata.StatusCode && doc.Metadata.StatusCode <= 399:
		redirectedURL := doc.Metadata.Headers.Get("Location")
		parsedRedirectedURL, err := url.Parse(redirectedURL)
		if err != nil {
			return err
		}
		if isDirectoryRedirect(u, parsedRedirectedURL) {
			return nil
		}
		return fmt.Errorf("redirect unsupported %q→%q", doc.Metadata.URL, redirectedURL)
	default:
		return fmt.Errorf("unsupported status code %d: %s", doc.Metadata.StatusCode, doc.Metadata.URL)
	}
}

var htmlExtensionRe = regexp.MustCompile(`\.[Hh][Tt][Mm][Ll]?$`)

func resolvePort(scheme, port string) string {
	if port != "" {
		return port
	}
	switch scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func isDirectoryRedirect(oldURL, newURL *url.URL) bool {
	return urlnorm.Canonical(oldURL).String()+"/" == urlnorm.Canonical(newURL).String()
}
