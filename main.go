package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"os"
	"site-to-static/httrack"
	"site-to-static/repository"
	"site-to-static/scraper"
	"site-to-static/urlnorm"
	"sort"
	"strings"

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

	repo := repository.New(repoPath)
	sc := scraper.Scraper{
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

type entry interface {
	CanonicalURL() string
	Body() ([]byte, error)
}

type repoEntry struct {
	e            repository.Entry
	canonicalURL string
}

func (r *repoEntry) CanonicalURL() string {
	return r.canonicalURL
}

func (r *repoEntry) Body() ([]byte, error) {
	doc, err := r.e.Open()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(doc.Body())
	closeErr := doc.Close()
	if err != nil {
		return nil, err
	}
	return data, closeErr
}

type httrackEntry struct {
	e            *httrack.Entry
	canonicalURL string
}

func (h *httrackEntry) CanonicalURL() string {
	return h.canonicalURL
}

func (h *httrackEntry) Body() ([]byte, error) {
	r, err := h.e.Body()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(r)
	closeErr := r.Close()
	if err != nil {
		return nil, err
	}
	return data, closeErr
}

func doDiff(c *cli.Context) error {
	if c.Args().Len() < 2 {
		return fmt.Errorf("not enough arguments")
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
			aData, err := entriesA[i].Body()
			if err != nil {
				return err
			}
			bData, err := entriesB[j].Body()
			if err != nil {
				return err
			}
			if bytes.Equal(aData, bData) {
				fmt.Printf("equal: %s\n", entriesA[i].CanonicalURL())
			} else {
				if isBinaryData(aData) || isBinaryData(bData) {
					fmt.Printf("binary files different (%d bytes vs %d bytes): %s\n",
						len(aData), len(bData), entriesA[i].CanonicalURL())
				} else {
					err = difflib.WriteUnifiedDiff(os.Stdout, difflib.UnifiedDiff{
						A:        splitLines(aData),
						FromFile: "a:" + entriesA[i].CanonicalURL(),
						B:        splitLines(bData),
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
