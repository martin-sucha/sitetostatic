// Package urlrebase rewrites URLs from using one base to another.
package urlrebase

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/martin-sucha/site-to-static/urlnorm"
)

// ErrNoBase is returned when rebase is called with an URL that is not under oldBase.
var ErrNoBase error = errors.New("base is not a parent of url")

// Rebase rewrites URL to be under different base.
// oldBase and newBase must be absolute URLs.
func Rebase(u, oldBase, newBase *url.URL) (*url.URL, error) {
	if !u.IsAbs() {
		// TODO: support relative URLs
		return nil, ErrNoBase
	}
	u = urlnorm.Canonical(u)
	oldBase = urlnorm.Canonical(oldBase)
	newBase = urlnorm.Canonical(newBase)

	if u.Scheme != oldBase.Scheme {
		return nil, ErrNoBase
	}
	u.Scheme = newBase.Scheme

	if u.Host != oldBase.Host {
		return nil, ErrNoBase
	}
	u.Host = newBase.Host

	if !strings.HasSuffix(oldBase.Path, "/") {
		if u.Path != oldBase.Path {
			return nil, ErrNoBase
		}
		u.Path = newBase.Path
	} else {
		if !strings.HasPrefix(u.Path, oldBase.Path) {
			return nil, ErrNoBase
		}
		if !strings.HasSuffix(newBase.Path, "/") {
			return nil, fmt.Errorf("if old base path ends with slash, new one must too")
		}
		u.Path = newBase.Path + u.Path[len(oldBase.Path):]
	}
	return u, nil
}
