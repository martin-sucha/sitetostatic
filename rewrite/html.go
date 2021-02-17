package rewrite

import (
	"errors"
	"fmt"
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/html"
	stdhtml "html"
	"io"
	"strings"
)

// Rewrite HTML5 page present in data, replace links with the result of urlRewriter and write output to w.
func HTML5(input *parse.Input, w io.Writer, urlRewriter URLRewriter) error {
	lexer := html.NewLexer(input)
	var currentTag []byte
	startPos := input.Offset()
	for {
		tt, data := lexer.Next()
		if tt == html.ErrorToken {
			break
		}
		endPos := input.Offset()
		copyInput := true
		fmt.Printf("%s %q\n", tt.String(), data)
		switch tt {
		case html.StartTagToken:
			fmt.Printf("%q\n", data)
			currentTag = lexer.Text()
		case html.StartTagCloseToken, html.StartTagVoidToken:
			currentTag = nil
		case html.AttributeToken:
			// TODO: handle meta http-equiv=refresh
			err := rewriteAttribute(currentTag, lexer.Text(), lexer.AttrVal(), w, urlRewriter)
			switch {
			case err == ErrNotModified:
				copyInput = true
			case err != nil:
				return err
			default:
				copyInput = false
			}
		case html.TextToken:
			fmt.Printf("text %q\n", data)
		}
		if copyInput {
			_, err := w.Write(input.Bytes()[startPos:endPos])
			if err != nil {
				return err
			}
		}
		startPos = endPos
	}
	err := lexer.Err()
	if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// rewriteAttribute either writes new attribute version to w or returns ErrNotModified.
func rewriteAttribute(tagName, attrName, attrValue []byte, w io.Writer, urlRewriter URLRewriter) error {
	handler := findHandler(tagName, attrName)
	if handler == nil {
		return ErrNotModified
	}

	var outputQuoteType byte
	var value []byte
	if len(attrValue) > 0 && (attrValue[0] == '\'' || attrValue[0] == '"') {
		if len(attrValue) < 2 {
			return fmt.Errorf("attribute %q does not have ending quote", string(attrValue))
		}
		startQuote := attrValue[0]
		endQuote := attrValue[len(attrValue)-1]
		if startQuote != endQuote {
			return fmt.Errorf("attribute quote mismatch %q vs %q", string(startQuote), string(endQuote))
		}
		// quoted attribute in input
		outputQuoteType = startQuote
		value = attrValue[1:len(attrValue)-1]
	} else {
		// unquoted attribute in input, output is always quoted
		outputQuoteType = '"'
		value = attrValue
	}

	cleanValue := stdhtml.UnescapeString(string(value))
	newString, err := handler(cleanValue, urlRewriter)
	if err != nil {
		return err
	}
	newBytes := []byte(stdhtml.EscapeString(newString))

	return multiWrite(w, attrName, []byte{'=', outputQuoteType}, newBytes, []byte{outputQuoteType})
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
	"cite": tags("blockquote", "del", "ins", "q"),
	"classid": tags("object"),
	"codebase": tags("applet", "object"),
	"data": tags("object"),
	"formaction": tags("button", "input"),
	"href": tags("a", "area", "link"), // ignore base for now
	"icon": tags("command"),
	"longdesc": tags("img", "frame", "iframe"),
	"manifest": tags("html"),
	"poster": tags("video"),
	"profile": tags("head"),
	"src": tags("audio", "embed", "iframe", "img", "input", "script", "source", "track", "video", "frame"),
	"srcset": {
		"img": srcSetAttribute,
		"source": srcSetAttribute,
	},
	"usemap": tags("img", "input", "object"),
}

type attrHandler func(attrValue string, urlRewriter URLRewriter) (string, error)

func urlAttribute(attrValue string, urlRewriter URLRewriter) (string, error) {
	return urlRewriter(attrValue)
}

func urlListAttribute(separator string) attrHandler {
	return func(attrValue string, urlRewriter URLRewriter) (string, error) {
		var buf strings.Builder
		parts := strings.Split(attrValue, separator)
		anyModified := false
		for i, part := range parts {
			if i > 0 {
				buf.WriteString(separator)
			}
			rewritten, err := urlRewriter(part)
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

func srcSetAttribute(attrValue string, urlRewriter URLRewriter) (string, error) {
	var buf strings.Builder
	parts := strings.Split(attrValue, ",")
	anyModified := false
	for i, part := range parts {
		if i > 0 {
			buf.WriteString(",")
		}
		trimmedPart := strings.TrimSpace(part)
		parts2 := strings.SplitN(trimmedPart, " ", 2)
		if len(trimmedPart) > 0 && len(parts2) > 0 {
			rewritten, err := urlRewriter(parts2[0])
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