package scraper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"runtime/pprof"
	"site-to-static/repository"
	"site-to-static/rewrite"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tdewolff/parse/v2"
	"golang.org/x/time/rate"
)

type Scraper struct {
	Client     http.Client
	Repository *repository.Repository
	Limiter    *rate.Limiter
	// FollowURL determines whether to scrape u or not.
	FollowURL func(u *url.URL) bool
	UserAgent string
}

func (s *Scraper) Scrape(initialURLs []*url.URL, workerCount int) {
	inTasks := make(chan *task)
	doneTasks := make(chan *task)
	outTasks := make(chan *task)
	initialTasks := make([]*task, 0, len(initialURLs))
	for _, u := range initialURLs {
		initialTasks = append(initialTasks, &task{
			downloadURL: u,
			key:         repository.Key(u),
		})
	}
	go func() {
		defer close(inTasks)
		defer close(doneTasks)
		defer close(outTasks)
		queue(initialTasks, inTasks, doneTasks, outTasks)
	}()

	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		labels := pprof.Labels("scraper-worker", strconv.Itoa(i))
		go pprof.Do(context.Background(), labels, func(_ context.Context) {
			defer wg.Done()
			for t := range outTasks {
				err := s.scrapeTask(t, inTasks, doneTasks)
				if err != nil {
					log.Println(err)
				}
			}
		})
	}

	wg.Wait()
}

func (s *Scraper) scrapeTask(t *task, newTasks, doneTasks chan<- *task) (errOut error) {
	defer func() {
		doneTasks <- t
	}()
	err := s.Limiter.Wait(context.TODO())
	if err != nil {
		return err
	}
	startTime := time.Now()
	client := s.Client
	originalCheckRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.Response != nil {
			err := s.processResponse(req.Response, startTime, newTasks)
			if err != nil {
				return err
			}
		}
		if originalCheckRedirect == nil {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		}
		return originalCheckRedirect(req, via)
	}
	req, err := http.NewRequest(http.MethodGet, t.downloadURL.String(), nil)
	if err != nil {
		return err
	}
	if s.UserAgent != "" {
		req.Header.Set("User-Agent", s.UserAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	return s.processResponse(resp, startTime, newTasks)
}

func (s *Scraper) processResponse(resp *http.Response, startTime time.Time, newTasks chan<- *task) error {
	supportedContentType := false
	mediatype, params, err := mime.ParseMediaType(resp.Header.Get("content-type"))
	if err == nil {
		supportedContentType = isSupportedMediaType(mediatype, params)
	}
	data, err := s.storeResponse(resp, startTime, supportedContentType)
	if err != nil {
		return err
	}
	if !supportedContentType {
		return nil
	}
	rewriter := func(u rewrite.URL) (string, error) {
		referenceURL, err := url.Parse(strings.TrimSpace(u.Value))
		if err != nil {
			log.Printf("parsing url in document %q: %v", resp.Request.URL.String(), err)
			return "", nil
		}
		baseURL := resp.Request.URL
		if u.Base != "" {
			baseURL, err = url.Parse(u.Base)
			if err != nil {
				return "", fmt.Errorf("parsing base url in document %q: %v", resp.Request.URL.String(), err)
			}
		}
		absoluteURL := baseURL.ResolveReference(referenceURL)
		if s.FollowURL == nil || !s.FollowURL(absoluteURL) {
			return "", rewrite.ErrNotModified
		}
		key := repository.Key(absoluteURL)
		newTasks <- &task{
			downloadURL: absoluteURL,
			key:         key,
		}
		return "", rewrite.ErrNotModified
	}

	switch mediatype {
	case "text/html":
		return rewrite.HTML5(parse.NewInputBytes(data), ioutil.Discard, rewriter)
	case "text/css":
		return rewrite.CSS(parse.NewInputBytes(data), ioutil.Discard, rewriter, false)
	default:
		return fmt.Errorf("unsupported media type: %s", mediatype)
	}
}

// isSupportedMediaType returns whether the given media type (as returned from mime.ParseMediaType) is supported.
func isSupportedMediaType(mediaType string, params map[string]string) bool {
	if mediaType != "text/html" && mediaType != "text/css" {
		return false
	}
	return params["charset"] == "" || strings.EqualFold(params["charset"], "utf-8")
}

func (s *Scraper) storeResponse(resp *http.Response, startTime time.Time,
	loadToMemory bool) (dataOut []byte, errOut error) {
	defer func() {
		closeErr := resp.Body.Close()
		if errOut == nil {
			errOut = closeErr
		}
	}()
	meta := &repository.DocumentMetadata{
		Key:                 repository.Key(resp.Request.URL),
		DownloadStartedTime: startTime,
		URL:                 resp.Request.URL.String(),
		Headers:             resp.Header,
	}
	var buf bytes.Buffer
	var bodyReader io.Reader = resp.Body
	if loadToMemory {
		bodyReader = io.TeeReader(bodyReader, &buf)
	}
	err := s.Repository.Store(meta, bodyReader)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
