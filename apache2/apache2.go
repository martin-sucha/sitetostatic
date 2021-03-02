// Package apache2 implements config generator to serve the site using apache2 server.
package apache2

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"site-to-static/apache2/internal/a2cfg"
	"site-to-static/repository"
	"site-to-static/urlnorm"
)

type Options struct {
	// DataRootPath is the path where the documents are stored on the apache2 server.
	DataRootPath string `yaml:"data_root_path"`
	// OutputDir is path where generated files are stored.
	OutputDir string `yaml:"output_dir"`
}

func Generate(repo *repository.Repository, opts Options) error {
	entries, err := repo.List()
	if err != nil {
		return err
	}
	err = os.Mkdir(opts.OutputDir, 0777)
	if err != nil {
		return err
	}
	err = os.Mkdir(filepath.Join(opts.OutputDir, "data"), 0777)
	if err != nil {
		return err
	}
	cg := &configGenerator{
		cfg:    &a2cfg.Config{},
		vhosts: make(map[vhostKey]*a2cfg.VirtualHost),
		repo:   repo,
		opts:   opts,
	}
	for _, e := range entries {
		doc, err := e.Open()
		if err != nil {
			return err
		}
		err = cg.processEntry(doc)
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

func (cg *configGenerator) processEntry(doc *repository.Document) error {
	u, err := url.Parse(doc.Metadata.URL)
	if err != nil {
		return err
	}
	uc := urlnorm.Canonical(u)
	vhostK := vhostKeyFromURL(uc)
	vhost := cg.getOrCreateVhost(vhostK)
	switch {
	case doc.Metadata.StatusCode == 404:
		// skip
		return nil
	case doc.Metadata.StatusCode == 200:
		if u.RawQuery != "" {
			// We can't use Alias to match query parameters, use Rewrite instead.
			fmt.Printf("TODO: %q\n", u.String())
			return nil
		}
		vhost.Aliases = append(vhost.Aliases, &a2cfg.Alias{
			URLPath:  u.Path,
			FilePath: path.Join(cg.opts.DataRootPath, u.Path),
		})
	case 300 <= doc.Metadata.StatusCode && doc.Metadata.StatusCode <= 399:
		redirectedURL := doc.Metadata.Headers.Get("Location")
		vhost.RedirectMatches = append(vhost.RedirectMatches, &a2cfg.RedirectMatch{
			Status: doc.Metadata.StatusCode,
			Regex:  `^` + pcreEscaper.Replace(u.Path) + `$`,
			URL:    redirectedURL,
		})
	default:
		fmt.Fprintf(os.Stderr, "unhandled status code %d: %s\n", doc.Metadata.StatusCode, doc.Metadata.URL)
	}
	return nil
}

type configGenerator struct {
	cfg    *a2cfg.Config
	vhosts map[vhostKey]*a2cfg.VirtualHost
	repo   *repository.Repository
	opts   Options
}

func (cg *configGenerator) getOrCreateVhost(key vhostKey) *a2cfg.VirtualHost {
	if vhost, ok := cg.vhosts[key]; ok {
		return vhost
	}
	vhost := &a2cfg.VirtualHost{
		ServerName: key.name,
		Port:       key.port,
	}
	cg.vhosts[key] = vhost
	return vhost
}

func vhostKeyFromURL(u *url.URL) vhostKey {
	k := vhostKey{
		name: u.Hostname(),
		port: u.Port(),
	}
	if k.port == "" {
		switch u.Scheme {
		case "http":
			k.port = "80"
		case "https":
			k.port = "443"
		}
	}
	return k
}

type vhostKey struct {
	name string
	port string
}
