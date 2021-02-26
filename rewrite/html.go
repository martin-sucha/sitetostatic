package rewrite

import (
	"bytes"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"regexp"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/html"
)

// Rewrite HTML5 page present in data, replace links with the result of urlRewriter and write output to w.
func HTML5(input *parse.Input, w io.Writer, urlRewriter URLRewriter) error {
	lc := html5Rewriter{
		input:       input,
		lexer:       html.NewLexer(input),
		w:           w,
		urlRewriter: urlRewriter,
	}
	for {
		tt, _ := lc.next()
		if tt == html.ErrorToken {
			return ignoreEOF(lc.err())
		}
		switch tt {
		case html.StartTagToken:
			currentTag := lc.text()
			err := lc.copy()
			if err != nil {
				return err
			}
			err = lc.processTag(currentTag)
			if err != nil {
				return err
			}
		default:
			err := lc.copy()
			if err != nil {
				return err
			}
		}
	}
}

func (lc *html5Rewriter) processTag(currentTag []byte) error {
	switch {
	case bytes.Equal(currentTag, []byte("meta")):
		return lc.processMeta()
	case bytes.Equal(currentTag, []byte("base")):
		return lc.rewriteAttributes(currentTag, func(tagName, attrName []byte) attrHandler {
			if !bytes.Equal(attrName, []byte("href")) {
				return nil
			}
			return baseHrefAttribute
		})
	default:
		return lc.rewriteAttributes(currentTag, findHandler)
	}
}

func ignoreEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func (lc *html5Rewriter) processMeta() error {
	attrs, closeTagRaw, err := lc.readAttributes()
	if err != nil {
		return err
	}
	var flags metaFlag
	var itemProp string

	for _, attr := range attrs {
		if bytes.Equal(attr.attrName, []byte("http-equiv")) {
			flags |= metaFlagRefresh
		} else if bytes.Equal(attr.attrName, []byte("itemprop")) || bytes.Equal(attr.attrName, []byte("property")) {
			flags |= metaFlagItemProp
			_, cleanValue, err := attr.cleanValue()
			if err != nil {
				return err
			}
			itemProp = cleanValue
		}
	}

	switch flags {
	case metaFlagRefresh:
		for _, attr := range attrs {
			if bytes.Equal(attr.attrName, []byte("content")) {
				err := attr.rewrite(lc, httpEquivRefreshAttribute)
				if err != nil {
					return err
				}
			} else {
				_, err := lc.w.Write(attr.rawData)
				if err != nil {
					return err
				}
			}
		}
	case metaFlagItemProp:
		if isOpenGraphURLProperty(itemProp) {
			for _, attr := range attrs {
				if bytes.Equal(attr.attrName, []byte("content")) {
					err := attr.rewrite(lc, openGraphContentAttribute)
					if err != nil {
						return err
					}
				} else {
					_, err := lc.w.Write(attr.rawData)
					if err != nil {
						return err
					}
				}
			}
		} else {
			for _, attr := range attrs {
				_, err := lc.w.Write(attr.rawData)
				if err != nil {
					return err
				}
			}
		}
	default:
		for _, attr := range attrs {
			_, err := lc.w.Write(attr.rawData)
			if err != nil {
				return err
			}
		}
	}
	_, err = lc.w.Write(closeTagRaw)
	return err
}

type metaFlag uint8

const (
	metaFlagRefresh = 1 << iota
	metaFlagItemProp
)

func (lc *html5Rewriter) readAttributes() ([]attributeToken, []byte, error) {
	attributes := make([]attributeToken, 0, 10)
	for {
		tt, data := lc.next()
		switch tt {
		case html.AttributeToken:
			attributes = append(attributes, attributeToken{
				data:      data,
				rawData:   lc.rawData(),
				attrName:  lc.text(),
				attrValue: lc.attrVal(),
			})
		case html.StartTagCloseToken, html.StartTagVoidToken:
			return attributes, lc.rawData(), nil
		case html.ErrorToken:
			return attributes, nil, lc.err()
		default:
			return attributes, nil, fmt.Errorf("unexpected token: %s", tt.String())
		}
	}
}

// rewriteAttributes rewrites tag's attributes in place.
func (lc *html5Rewriter) rewriteAttributes(tagName []byte, findHandlerFunc findHandlerFunc) error {
	for {
		tt, data := lc.next()
		switch tt {
		case html.AttributeToken:
			handler := findHandlerFunc(tagName, lc.text())
			if handler == nil {
				return lc.copy()
			}
			attr := attributeToken{
				data:      data,
				rawData:   lc.rawData(),
				attrName:  lc.text(),
				attrValue: lc.attrVal(),
			}
			err := attr.rewrite(lc, handler)
			if err != nil {
				return err
			}
		case html.StartTagCloseToken, html.StartTagVoidToken:
			return lc.copy()
		case html.ErrorToken:
			return lc.err()
		default:
			return fmt.Errorf("unexpected token: %s", tt.String())
		}
	}
}

type html5Rewriter struct {
	input               *parse.Input
	lexer               *html.Lexer
	w                   io.Writer
	startPos, endPos    int
	baseURL, newBaseURL string
	baseURLSet          bool
	urlRewriter         URLRewriter
}

func (lc *html5Rewriter) next() (html.TokenType, []byte) {
	lc.startPos = lc.input.Offset()
	tt, data := lc.lexer.Next()
	lc.endPos = lc.input.Offset()
	return tt, data
}

func (lc *html5Rewriter) text() []byte {
	return lc.lexer.Text()
}

func (lc *html5Rewriter) attrVal() []byte {
	return lc.lexer.AttrVal()
}

func (lc *html5Rewriter) copy() error {
	_, err := lc.w.Write(lc.rawData())
	return err
}

func (lc *html5Rewriter) rawData() []byte {
	return lc.input.Bytes()[lc.startPos:lc.endPos]
}

func (lc *html5Rewriter) err() error {
	return lc.lexer.Err()
}

type attributeToken struct {
	data      []byte
	rawData   []byte
	attrName  []byte
	attrValue []byte
}

func (at *attributeToken) copy(w io.Writer) error {
	_, err := w.Write(at.rawData)
	return err
}

func (at *attributeToken) cleanValue() (byte, string, error) {
	var outputQuoteType byte
	var value []byte
	if len(at.attrValue) > 0 && (at.attrValue[0] == '\'' || at.attrValue[0] == '"') {
		if len(at.attrValue) < 2 {
			return 0, "", fmt.Errorf("attribute %q does not have ending quote", string(at.attrValue))
		}
		startQuote := at.attrValue[0]
		endQuote := at.attrValue[len(at.attrValue)-1]
		if startQuote != endQuote {
			return 0, "", fmt.Errorf("attribute quote mismatch %q vs %q", string(startQuote), string(endQuote))
		}
		// quoted attribute in input
		outputQuoteType = startQuote
		value = at.attrValue[1 : len(at.attrValue)-1]
	} else {
		// unquoted attribute in input, output is always quoted
		outputQuoteType = '"'
		value = at.attrValue
	}

	cleanValue := stdhtml.UnescapeString(string(value))
	return outputQuoteType, cleanValue, nil
}

// rewriteAttribute either writes new attribute version to w.
func (at *attributeToken) rewrite(lc *html5Rewriter, handler attrHandler) error {
	outputQuoteType, cleanValue, err := at.cleanValue()
	if err != nil {
		return err
	}

	newString, err := handler(lc, cleanValue)
	switch {
	case errors.Is(err, ErrNotModified):
		return at.copy(lc.w)
	case err != nil:
		return err
	}
	newBytes := []byte(stdhtml.EscapeString(newString))

	return multiWrite(lc.w, at.data[0:len(at.data)-len(at.attrValue)], []byte{outputQuoteType}, newBytes,
		[]byte{outputQuoteType})
}

func multiWrite(w io.Writer, bufs ...[]byte) error {
	for _, buf := range bufs {
		_, err := w.Write(buf)
		if err != nil {
			return err
		}
	}
	return nil
}

func tags(tagNames ...string) map[string]attrHandler {
	m := make(map[string]attrHandler, len(tagNames))
	for _, tagName := range tagNames {
		m[tagName] = urlAttribute
	}
	return m
}

// findHandler returns handler for the attribute or nil.
func findHandler(tagName, attrName []byte) attrHandler {
	attr := attributeHandlers[string(attrName)]
	if attr == nil {
		return nil
	}
	return attr[string(tagName)]
}

type findHandlerFunc func(tagName, attrName []byte) attrHandler

// attributeHandlers is a map[attrName]map[tagName]attrHandler
// based on:
//
//  - https://www.w3.org/TR/2017/REC-html52-20171214/fullindex.html#attributes-table
//  - https://html.spec.whatwg.org/index.html#attributes-1
//  - https://stackoverflow.com/a/2725168
var attributeHandlers = map[string]map[string]attrHandler{
	"action": tags("form"),
	"archive": {
		"object": urlListAttribute(" "),
		"applet": urlListAttribute(","),
	},
	"background": tags("body"),
	"cite":       tags("blockquote", "del", "ins", "q"),
	"classid":    tags("object"),
	"codebase":   tags("applet", "object"),
	"data":       tags("object"),
	"formaction": tags("button", "input"),
	"href":       tags("a", "area", "link"), // ignore base for now
	"icon":       tags("command"),
	"longdesc":   tags("img", "frame", "iframe"),
	"manifest":   tags("html"),
	"poster":     tags("video"),
	"profile":    tags("head"),
	"src":        tags("audio", "embed", "iframe", "img", "input", "script", "source", "track", "video", "frame"),
	"srcset": {
		"img":    srcSetAttribute,
		"source": srcSetAttribute,
	},
	"usemap": tags("img", "input", "object"),
}

type attrHandler func(lc *html5Rewriter, attrValue string) (string, error)

func urlAttribute(lc *html5Rewriter, attrValue string) (string, error) {
	return lc.urlRewriter(URL{
		Value:   attrValue,
		Base:    lc.baseURL,
		NewBase: lc.newBaseURL,
		Type:    URLTypeUnknown,
	})
}

func openGraphContentAttribute(lc *html5Rewriter, attrValue string) (string, error) {
	// OpenGraph URLs are always absolute, they don't obey base.
	// https://developer.mozilla.org/en-US/docs/Web/HTML/Element/base#open_graph
	return lc.urlRewriter(URL{
		Value:   attrValue,
		Base:    "",
		NewBase: "",
		Type:    URLTypeOpenGraph,
	})
}

func urlListAttribute(separator string) attrHandler {
	return func(lc *html5Rewriter, attrValue string) (string, error) {
		var buf strings.Builder
		parts := strings.Split(attrValue, separator)
		anyModified := false
		for i, part := range parts {
			if i > 0 {
				buf.WriteString(separator)
			}
			rewritten, err := lc.urlRewriter(URL{
				Value:   part,
				Base:    lc.baseURL,
				NewBase: lc.newBaseURL,
				Type:    URLTypeUnknown,
			})
			switch {
			case errors.Is(err, ErrNotModified):
				buf.WriteString(part)
			case err != nil:
				return "", err
			default:
				buf.WriteString(rewritten)
				anyModified = true
			}
		}
		if !anyModified {
			return "", ErrNotModified
		}
		return buf.String(), nil
	}
}

func srcSetAttribute(lc *html5Rewriter, attrValue string) (string, error) {
	var buf strings.Builder
	parts := strings.Split(attrValue, ",")
	anyModified := false
	for i, part := range parts {
		if i > 0 {
			buf.WriteString(", ")
		}
		trimmedPart := strings.TrimSpace(part)
		parts2 := strings.SplitN(trimmedPart, " ", 2)
		if len(trimmedPart) > 0 && len(parts2) > 0 {
			rewritten, err := lc.urlRewriter(URL{
				Value:   parts2[0],
				Base:    lc.baseURL,
				NewBase: lc.newBaseURL,
				Type:    URLTypeUnknown,
			})
			switch {
			case errors.Is(err, ErrNotModified):
				buf.WriteString(part)
			case err != nil:
				return "", err
			default:
				buf.WriteString(rewritten)
				if len(parts2) > 1 {
					buf.WriteString(" ")
					buf.WriteString(parts2[1])
				}
				anyModified = true
			}
		} else {
			buf.WriteString(part)
		}
	}
	if !anyModified {
		return "", ErrNotModified
	}
	return buf.String(), nil
}

var refreshRegexp = regexp.MustCompile(`^\s*(\d+)\s*(?:;url=(.*)\s*)?$`)

func httpEquivRefreshAttribute(lc *html5Rewriter, attrValue string) (string, error) {
	m := refreshRegexp.FindStringSubmatch(attrValue)
	if len(m) != 3 {
		return "", ErrNotModified
	}
	newURL, err := lc.urlRewriter(URL{
		Value:   m[2],
		Base:    lc.baseURL,
		NewBase: lc.newBaseURL,
		Type:    URLTypeUnknown,
	})
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString(m[1])
	sb.WriteString(";url=")
	sb.WriteString(newURL)
	return sb.String(), nil
}

func baseHrefAttribute(lc *html5Rewriter, attrValue string) (string, error) {
	if lc.baseURLSet {
		return "", ErrNotModified
	}
	lc.baseURL = attrValue
	newBaseURL, err := lc.urlRewriter(URL{Value: attrValue, Type: URLTypeBase})
	switch {
	case errors.Is(err, ErrNotModified):
		lc.newBaseURL = lc.baseURL
		return "", ErrNotModified
	case err != nil:
		return "", err
	default:
		lc.newBaseURL = newBaseURL
		return newBaseURL, nil
	}
}
