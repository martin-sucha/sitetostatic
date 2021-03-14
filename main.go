package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/martin-sucha/site-to-static/rewrite"
	"github.com/martin-sucha/site-to-static/urlrebase"

	"github.com/martin-sucha/site-to-static/files"
	"github.com/martin-sucha/site-to-static/httrack"
	"github.com/martin-sucha/site-to-static/repository"
	"github.com/martin-sucha/site-to-static/scraper"
	"github.com/martin-sucha/site-to-static/urlnorm"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/urfave/cli/v2"
	"golang.org/x/time/rate"
)

func main() {
	app := &cli.App{
		Name:  "scrape-to-static",
		Usage: "Scrape a website and convert it to a static site",
		Commands: []*cli.Command{
			{
				Name:      "scrape",
				Usage:     "",
				ArgsUsage: "repopath url [url...]",
				Action:    doScrape,
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "allow-root",
						Usage: "URL prefixes to allow",
					},
					&cli.StringFlag{
						Name:  "user-agent",
						Usage: "User-Agent string to use",
					},
					&cli.StringSliceFlag{
						Name:  "remap-address",
						Usage: "Format is orig_add|new_addr. Instead of connecting to orig_addr, connect to new_addr",
					},
					&cli.BoolFlag{
						Name:  "strip-https",
						Usage: "Use plain HTTP (without TLS) for https URLs",
					},
				},
			},
			{
				Name:      "list",
				Usage:     "list urls stored in a repository",
				ArgsUsage: "repopath",
				Action:    doList,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "format",
						Usage: "either native or httrack",
					},
					&cli.BoolFlag{
						Name:  "canonical",
						Usage: "print canonical URLs",
					},
				},
			},
			{
				Name:      "diff",
				Usage:     "Diff two repositories",
				ArgsUsage: "repopath-a repopath-b",
				Action:    doDiff,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "a-format",
						Usage: "either native or httrack",
					},
					&cli.StringFlag{
						Name:  "b-format",
						Usage: "either native or httrack",
					},
					&cli.StringFlag{
						Name:  "ignore-status",
						Usage: "Don't show diff if both have same status code from this list",
					},
					&cli.BoolFlag{
						Name:  "headers",
						Usage: "Show diff of headers",
					},
				},
			},
			{
				Name:      "show",
				Usage:     "show url stored in a repository",
				ArgsUsage: "repopath url",
				Action:    doShow,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "format",
						Usage: "either native or httrack",
					},
				},
			},
			{
				Name:      "files",
				Usage:     "copy files to directory",
				ArgsUsage: "repopath outdir",
				Action:    doFiles,
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "rewrite-url",
						Usage: "oldURL|newURL",
					},
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func doScrape(c *cli.Context) error {
	if c.Args().Len() < 2 {
		return fmt.Errorf("not enough arguments")
	}
	repoPath := c.Args().First()
	initialURLArgs := c.Args().Tail()
	initialURLs := make([]*url.URL, 0, len(initialURLArgs))
	for _, arg := range initialURLArgs {
		u, err := url.Parse(arg)
		if err != nil {
			return fmt.Errorf("parse initial url %q: %v", arg, err)
		}
		initialURLs = append(initialURLs, u)
	}

	rootStrings := c.StringSlice("allow-root")
	rootKeys := make([]string, 0, len(rootStrings))
	for _, arg := range rootStrings {
		u, err := url.Parse(arg)
		if err != nil {
			return fmt.Errorf("parse root url %q: %v", arg, err)
		}
		rootKeys = append(rootKeys, repository.Key(u))
	}

	var httpClient http.Client
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	remapAddresses := c.StringSlice("remap-address")
	if len(remapAddresses) > 0 {
		remapAddressesMap := make(map[string]string, len(remapAddresses))
		for _, mapping := range remapAddresses {
			mappingParts := strings.SplitN(mapping, "|", 2)
			if len(mappingParts) != 2 {
				return fmt.Errorf("parse address mapping %q: | not found", mapping)
			}
			remapAddressesMap[strings.ToLower(mappingParts[0])] = mappingParts[1]
		}
		dialContext := func(ctx context.Context, network, address string) (net.Conn, error) {
			if newAddress, ok := remapAddressesMap[strings.ToLower(address)]; ok {
				return dialer.DialContext(ctx, network, newAddress)
			}
			return dialer.DialContext(ctx, network, address)
		}
		transport.DialContext = dialContext
	}
	httpClient.Transport = transport

	if c.Bool("strip-https") {
		httpClient.Transport = &stripHTTPSRoundTripper{rt: httpClient.Transport}
	}

	repo := repository.New(repoPath)
	sc := scraper.Scraper{
		Client:     httpClient,
		Repository: repo,
		Limiter:    rate.NewLimiter(10, 1),
		FollowURL: func(u *url.URL) bool {
			key := repository.Key(u)
			for _, root := range rootKeys {
				if strings.HasPrefix(key, root) {
					return true
				}
			}
			return false
		},
		UserAgent: c.String("user-agent"),
	}
	sc.Scrape(initialURLs, 10)
	return nil
}

type stripHTTPSRoundTripper struct {
	rt http.RoundTripper
}

func (s *stripHTTPSRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if strings.ToLower(request.URL.Scheme) != "https" {
		return s.rt.RoundTrip(request)
	}
	newURL := new(url.URL)
	*newURL = *request.URL
	newURL.Scheme = "http"
	newRequest := new(http.Request)
	*newRequest = *request
	newRequest.URL = newURL
	resp, err := s.rt.RoundTrip(newRequest)
	if resp != nil {
		// Update request so that we store the original URL in cache.
		resp.Request = request
	}
	return resp, err
}

func doList(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("not enough arguments")
	}
	format := c.String("format")

	printURLFunc := func(u string) error {
		_, err := fmt.Println(u)
		return err
	}
	if c.Bool("canonical") {
		printURLFunc = func(u string) error {
			parsedURL, err := url.Parse(u)
			if err != nil {
				return err
			}
			_, err = fmt.Println(urlnorm.Canonical(parsedURL).String())
			return err
		}
	}

	repoPath := c.Args().First()
	switch format {
	case "", "native":
		repo := repository.New(repoPath)
		entries, err := repo.List()
		if err != nil {
			return err
		}
		for _, entry := range entries {
			doc, err := entry.Open()
			if err != nil {
				return err
			}
			err = printURLFunc(doc.Metadata.URL)
			if err != nil {
				return err
			}
			err = doc.Close()
			if err != nil {
				return err
			}
		}
	case "httrack":
		cache, err := httrack.OpenCache(repoPath)
		if err != nil {
			return err
		}
		for _, entry := range cache.Entries {
			err = printURLFunc(entry.URL)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

type entryData struct {
	Response *http.Response
	Body     []byte
}

type entry interface {
	CanonicalURL() string
	Read() (entryData, error)
}

type repoEntry struct {
	e            repository.Entry
	canonicalURL string
}

func (r *repoEntry) CanonicalURL() string {
	return r.canonicalURL
}

func (r *repoEntry) Read() (entryData, error) {
	doc, err := r.e.Open()
	if err != nil {
		return entryData{}, err
	}
	data, err := io.ReadAll(doc.Body())
	closeErr := doc.Close()
	if err != nil {
		return entryData{}, err
	}
	resp := &http.Response{
		Status:        doc.Metadata.Status,
		StatusCode:    doc.Metadata.StatusCode,
		Proto:         doc.Metadata.Proto,
		Header:        doc.Metadata.Headers,
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: doc.BodySize,
		Trailer:       doc.Metadata.Trailers,
	}
	ret := entryData{
		Response: resp,
		Body:     data,
	}
	return ret, closeErr
}

type httrackEntry struct {
	e            *httrack.Entry
	canonicalURL string
}

func (h *httrackEntry) CanonicalURL() string {
	return h.canonicalURL
}

func (h *httrackEntry) Read() (entryData, error) {
	r, err := h.e.Body()
	if err != nil {
		return entryData{}, err
	}
	data, err := io.ReadAll(r)
	closeErr := r.Close()
	if err != nil {
		return entryData{}, err
	}
	resp := &http.Response{
		Status:        h.e.Status,
		StatusCode:    h.e.StatusCode,
		Proto:         h.e.Proto,
		Header:        h.e.Header,
		ContentLength: h.e.Size,
		Body:          io.NopCloser(bytes.NewReader(data)),
	}
	ret := entryData{
		Response: resp,
		Body:     data,
	}
	return ret, closeErr
}

func doDiff(c *cli.Context) error {
	if c.Args().Len() < 2 {
		return fmt.Errorf("not enough arguments")
	}
	diffHeaders := c.Bool("headers")
	ignoreStatuses := make(map[int]struct{})
	if c.String("ignore-status") != "" {
		for _, val := range strings.Split(c.String("ignore-status"), ",") {
			sc, err := strconv.Atoi(val)
			if err != nil {
				return fmt.Errorf("ignore-status can't parse %q: %v", val, err)
			}
			ignoreStatuses[sc] = struct{}{}
		}
	}
	entriesA, err := getEntries(c.Args().Get(0), c.String("a-format"))
	if err != nil {
		return err
	}
	entriesB, err := getEntries(c.Args().Get(1), c.String("b-format"))
	if err != nil {
		return err
	}
	sort.Slice(entriesA, func(i, j int) bool {
		return entriesA[i].CanonicalURL() < entriesA[j].CanonicalURL()
	})
	sort.Slice(entriesB, func(i, j int) bool {
		return entriesB[i].CanonicalURL() < entriesB[j].CanonicalURL()
	})
	i := 0
	j := 0
	for i < len(entriesA) || j < len(entriesB) {
		switch {
		case i >= len(entriesA):
			fmt.Printf("only in B: %s\n", entriesB[j].CanonicalURL())
			j++
		case j >= len(entriesB):
			fmt.Printf("only in A: %s\n", entriesA[i].CanonicalURL())
			i++
		case entriesA[i].CanonicalURL() == entriesB[j].CanonicalURL():
			aData, err := entriesA[i].Read()
			if err != nil {
				return err
			}
			bData, err := entriesB[j].Read()
			if err != nil {
				return err
			}
			ignoreDiff := false
			if aData.Response.StatusCode != bData.Response.StatusCode {
				fmt.Printf("status code differs %s: %d vs %d\n", entriesA[i].CanonicalURL(),
					aData.Response.StatusCode, bData.Response.StatusCode)
			} else if _, ok := ignoreStatuses[aData.Response.StatusCode]; ok {
				ignoreDiff = true
			}
			if diffHeaders && !ignoreDiff {
				aHeaders, err := headerLines(aData.Response)
				if err != nil {
					return err
				}
				bHeaders, err := headerLines(bData.Response)
				if err != nil {
					return err
				}
				err = difflib.WriteUnifiedDiff(os.Stdout, difflib.UnifiedDiff{
					A:        aHeaders,
					FromFile: "a (headers): " + entriesA[i].CanonicalURL(),
					B:        bHeaders,
					ToFile:   "b (headers): " + entriesB[j].CanonicalURL(),
					Eol:      "\n",
				})
				if err != nil {
					return err
				}
			}
			if ignoreDiff {
				fmt.Printf("ignored body: %s\n", entriesA[i].CanonicalURL())
			} else if bytes.Equal(aData.Body, bData.Body) {
				fmt.Printf("equal: %s\n", entriesA[i].CanonicalURL())
			} else {
				if isBinaryData(aData.Body) || isBinaryData(bData.Body) {
					fmt.Printf("binary files different (%d bytes vs %d bytes): %s\n",
						len(aData.Body), len(bData.Body), entriesA[i].CanonicalURL())
				} else {
					err = difflib.WriteUnifiedDiff(os.Stdout, difflib.UnifiedDiff{
						A:        splitLines(aData.Body),
						FromFile: "a:" + entriesA[i].CanonicalURL(),
						B:        splitLines(bData.Body),
						ToFile:   "b:" + entriesB[j].CanonicalURL(),
						Eol:      "\n",
					})
					if err != nil {
						return err
					}
					fmt.Println()
					fmt.Println()
				}
			}
			i++
			j++
		case entriesA[i].CanonicalURL() < entriesB[j].CanonicalURL():
			fmt.Printf("only in A: %s\n", entriesA[i].CanonicalURL())
			i++
		default:
			fmt.Printf("only in B: %s\n", entriesB[j].CanonicalURL())
			j++
		}
	}
	return nil
}

func isBinaryData(data []byte) bool {
	for i := 0; i < len(data); i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

func splitLines(data []byte) []string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	count := 0
	for scanner.Scan() {
		count++
	}
	lines := make([]string, 0, count)
	scanner = bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		var sb strings.Builder
		sb.Grow(len(scanner.Bytes()))
		sb.Write(scanner.Bytes())
		sb.WriteRune('\n')
		lines = append(lines, sb.String())
	}
	return lines
}

func headerLines(resp *http.Response) ([]string, error) {
	data, err := httputil.DumpResponse(resp, false)
	if err != nil {
		return nil, err
	}
	lines := splitLines(data)
	if len(lines) < 1 {
		return nil, fmt.Errorf("unexpected response serialization with no lines")
	}
	return lines[1:], nil
}

func getEntries(repoPath, format string) ([]entry, error) {
	switch format {
	case "", "native":
		repo := repository.New(repoPath)
		entries, err := repo.List()
		if err != nil {
			return nil, err
		}
		out := make([]entry, 0, len(entries))
		for _, e := range entries {
			doc, err := e.Open()
			if err != nil {
				return nil, err
			}
			err = doc.Close()
			if err != nil {
				return nil, err
			}
			parsedURL, err := url.Parse(doc.Metadata.URL)
			if err != nil {
				return nil, err
			}
			out = append(out, &repoEntry{
				e:            e,
				canonicalURL: urlnorm.Canonical(parsedURL).String(),
			})
		}
		return out, nil
	case "httrack":
		cache, err := httrack.OpenCache(repoPath)
		if err != nil {
			return nil, err
		}
		out := make([]entry, 0, len(cache.Entries))
		for _, e := range cache.Entries {
			parsedURL, err := url.Parse(e.URL)
			if err != nil {
				return nil, err
			}
			out = append(out, &httrackEntry{
				e:            e,
				canonicalURL: urlnorm.Canonical(parsedURL).String(),
			})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported repo format: %s", format)
	}
}

func doShow(c *cli.Context) error {
	if c.Args().Len() < 2 {
		return fmt.Errorf("not enough arguments")
	}
	repoPath := c.Args().First()
	u := c.Args().Get(1)
	parsedURL, err := url.Parse(u)
	if err != nil {
		return err
	}
	if !parsedURL.IsAbs() {
		return fmt.Errorf("must be absolute url")
	}
	switch c.String("format") {
	case "", "native":
		repo := repository.New(repoPath)
		doc, err := repo.Load(repository.Key(parsedURL))
		if err != nil {
			return err
		}
		fmt.Printf("URL: %s\n", doc.Metadata.URL)
		fmt.Printf("Key: %s\n", doc.Metadata.Key)
		fmt.Printf("Download started: %s\n", doc.Metadata.DownloadStartedTime.Format(time.RFC3339))
		fmt.Println()
		resp := &http.Response{
			Status:        doc.Metadata.Status,
			StatusCode:    doc.Metadata.StatusCode,
			Proto:         doc.Metadata.Proto,
			Header:        doc.Metadata.Headers,
			Body:          io.NopCloser(doc.Body()),
			ContentLength: doc.BodySize,
			Trailer:       doc.Metadata.Trailers,
		}
		data, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return err
		}
		closeErr := doc.Close()
		_, err = os.Stdout.Write(data)
		if err != nil {
			return err
		}
		return closeErr
	case "httrack":
		cache, err := httrack.OpenCache(repoPath)
		if err != nil {
			return err
		}
		e := cache.FindEntry(func(e *httrack.Entry) bool {
			return e.URL == u
		})
		if e == nil {
			return fmt.Errorf("%q not found", u)
		}
		fmt.Printf("URL: %s\n", e.URL)
		fmt.Printf("In cache: %v\n", e.InCache)
		fmt.Println()
		body, err := e.Body()
		if err != nil {
			return err
		}
		resp := &http.Response{
			Status:        e.Status,
			StatusCode:    e.StatusCode,
			Proto:         e.Proto,
			Header:        e.Header,
			ContentLength: e.Size,
			Body:          body,
		}
		data, err := httputil.DumpResponse(resp, true)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(data)
		return err
	default:
		return fmt.Errorf("unsupported format: %s", c.String("format"))
	}
}

func doFiles(c *cli.Context) error {
	if c.Args().Len() < 2 {
		return fmt.Errorf("not enough arguments")
	}
	repoPath := c.Args().First()
	outputPath := c.Args().Get(1)
	repo := repository.New(repoPath)

	mappings, err := parseURLMapping(c)
	if err != nil {
		return err
	}

	var urlRewriter rewrite.URLRewriter
	if len(mappings) > 0 {
		urlRewriter = func(urlInfo rewrite.URL) (string, error) {
			parsedURL, err := url.Parse(strings.TrimSpace(urlInfo.Value))
			if err != nil {
				return "", err
			}
			for _, mapping := range mappings {
				newURL, err := urlrebase.Rebase(parsedURL, mapping.oldURL, mapping.newURL)
				switch {
				case errors.Is(err, urlrebase.ErrNoBase):
					continue
				case err != nil:
					return "", err
				default:
					return newURL.String(), nil
				}
			}
			return "", rewrite.ErrNotModified
		}
	}

	return files.Generate(repo, outputPath, urlRewriter)
}

func parseURLMapping(c *cli.Context) ([]urlMapping, error) {
	var mappings []urlMapping
	for _, s := range c.StringSlice("rewrite-url") {
		parts := strings.SplitN(s, "|", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("rewrite-url requires two pipe separated URLs")
		}
		oldURL, err := url.Parse(parts[0])
		if err != nil {
			return nil, err
		}
		newURL, err := url.Parse(parts[1])
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, urlMapping{
			oldURL: oldURL,
			newURL: newURL,
		})
	}
	return mappings, nil
}

type urlMapping struct {
	oldURL *url.URL
	newURL *url.URL
}
