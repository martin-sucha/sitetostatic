package rewrite

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tdewolff/parse/v2/css"

	"github.com/tdewolff/parse/v2"
)

func CSS(input *parse.Input, w io.Writer, rewriter URLRewriter, isInline bool) error {
	//p := css.NewParser(input, isInline)
	//for {
	//	gt, tt, data := p.Next()
	//	if gt == css.ErrorGrammar {
	//		return ignoreEOF(p.Err())
	//	}
	//
	//	switch gt {
	//	case css.AtRuleGrammar:
	//		if bytes.EqualFold(data, []byte("@import")) {
	//			parse
	//		} else {
	//
	//		}
	//	}
	//
	//	fmt.Printf("%s %s %q\n", gt.String(), tt.String(), string(data))
	//	for _, tok := range p.Values() {
	//		fmt.Printf("  %s %q\n", tok.TokenType, tok.Data)
	//	}
	//}
	l := css.NewLexer(input)
	lc := &cssRewriter{
		input:       input,
		lexer:       l,
		w:           w,
		urlRewriter: rewriter,
	}
	for {
		tt, text := lc.next()
		switch tt {
		case css.ErrorToken:
			return ignoreEOF(l.Err())
		case css.URLToken:
			err := lc.handleURLToken(text)
			if err != nil {
				return err
			}
		case css.AtKeywordToken:
			if bytes.EqualFold(text, []byte("@import")) {
				err := lc.processImport()
				if err != nil {
					return err
				}
			} else {
				err := lc.copy()
				if err != nil {
					return err
				}
			}
		default:
			err := lc.copy()
			if err != nil {
				return err
			}
		}
	}
}

type cssRewriter struct {
	input            *parse.Input
	lexer            *css.Lexer
	w                io.Writer
	startPos, endPos int
	urlRewriter      URLRewriter

	pushedBack bool
	tt         css.TokenType
	text       []byte
}

func (lc *cssRewriter) next() (css.TokenType, []byte) {
	if lc.pushedBack {
		lc.pushedBack = false
		return lc.tt, lc.text
	}
	lc.startPos = lc.input.Offset()
	tt, data := lc.lexer.Next()
	lc.endPos = lc.input.Offset()
	return tt, data
}

func (lc *cssRewriter) pushBack() {
	if lc.pushedBack {
		panic("a token is already stored")
	}
	lc.pushedBack = true
}

func (lc *cssRewriter) copy() error {
	_, err := lc.w.Write(lc.rawData())
	return err
}

func (lc *cssRewriter) rawData() []byte {
	return lc.input.Bytes()[lc.startPos:lc.endPos]
}

func (lc *cssRewriter) err() error {
	return lc.lexer.Err()
}

func (lc *cssRewriter) processImport() error {
	// copy the @import token
	err := lc.copy()
	if err != nil {
		return err
	}
	tt, _ := lc.next()
	switch tt {
	case css.ErrorToken:
		return lc.err()
	case css.WhitespaceToken:
		err = lc.copy()
		if err != nil {
			return err
		}
	default:
		// unexpected, go back to regular handling
		lc.pushBack()
		return nil
	}

	tt, text := lc.next()
	switch tt {
	case css.ErrorToken:
		return lc.err()
	case css.StringToken:
		value, size, err := cssUnescapeString(text)
		if err != nil {
			return err
		}
		if size != len(text) {
			return fmt.Errorf("string does not span whole string token")
		}
		newValue, err := lc.urlRewriter(URL{
			Value: value,
			Type:  URLTypeCSS,
		})
		switch {
		case errors.Is(err, ErrNotModified):
			return lc.copy()
		case err != nil:
			return err
		}
		escaped, err := cssEscapeString(newValue)
		if err != nil {
			return err
		}
		_, err = lc.w.Write(escaped)
		return err
	case css.URLToken:
		return lc.handleURLToken(text)
	default:
		// unexpected, go back to regular handling
		lc.pushBack()
		return nil
	}
}

func (lc *cssRewriter) handleURLToken(text []byte) error {
	if len(text) < 5 {
		return fmt.Errorf("unexpected token length for %q", text)
	}
	if !bytes.Equal(parse.ToLower(text[:4]), []byte("url(")) {
		return fmt.Errorf("unexpected token start for %q", text)
	}
	if text[len(text)-1] != ')' {
		return fmt.Errorf("unexpected token end for %q", text)
	}
	urlStartIndex := 4
	for urlStartIndex < len(text) && isWhiteSpace(rune(text[urlStartIndex])) {
		urlStartIndex++
	}
	if urlStartIndex >= len(text) {
		return fmt.Errorf("unexpected token end for %q", text)
	}
	var urlEndIndex int
	var urlValue string
	if text[urlStartIndex] == '"' || text[urlStartIndex] == '\'' {
		// quoted string
		unescaped, size, err := cssUnescapeString(text[urlStartIndex:])
		if err != nil {
			return err
		}
		urlEndIndex = urlStartIndex + size
		urlValue = unescaped
	} else {
		// unquoted url
		urlEndIndex = len(text) - 1
		for urlEndIndex > urlStartIndex && isWhiteSpace(rune(text[urlEndIndex-1])) {
			urlEndIndex--
		}
		urlValue = string(text[urlStartIndex:urlEndIndex])
	}
	newURL, err := lc.urlRewriter(URL{
		Value: urlValue,
		Type:  URLTypeCSS,
	})
	switch {
	case errors.Is(err, ErrNotModified):
		return lc.copy()
	case err != nil:
		return err
	default:
		escaped, err := cssEscapeString(newURL)
		if err != nil {
			return err
		}
		return multiWrite(lc.w, text[:urlStartIndex], escaped, text[urlEndIndex:])
	}
}

func cssEscapeString(value string) ([]byte, error) {
	// https://drafts.csswg.org/css-syntax-3/#consume-string-token
	var b bytes.Buffer
	b.Grow(len(value))
	b.WriteRune('"')
	idx := 0
Loop:
	for {
		r, size := utf8.DecodeRuneInString(value[idx:])
		switch {
		case r == utf8.RuneError && size == 0:
			// Empty string.
			break Loop
		case r == utf8.RuneError && size == 1:
			// Invalid utf8 data
			return nil, fmt.Errorf("css: escape string: invalid utf8 data")
		case r == '\n' || r == '"' || r == '\\':
			// Needs escape.
			b.WriteRune('\\')
			b.WriteString(strconv.FormatInt(int64(r), 16))
			b.WriteRune(' ')
		default:
			// Copy verbatim.
			b.WriteString(value[idx : idx+size])
		}
		idx += size
	}
	b.WriteRune('"')
	return b.Bytes(), nil
}

// cssUnescapeString returns unescaped string value and number of bytes consumed.
func cssUnescapeString(data []byte) (string, int, error) {
	origData := data
	// https://drafts.csswg.org/css-syntax-3/#consume-string-token
	quote, size := utf8.DecodeRune(data)
	if !(quote == '"' || quote == '\'') {
		return "", 0, fmt.Errorf("unexpected rune instead of quote: %c", quote)
	}
	data = data[size:]
	var sb strings.Builder
	for {
		r, size := utf8.DecodeRune(data)
		data = data[size:]
		switch {
		case r == utf8.RuneError && size == 0:
			return "", len(origData) - len(data), fmt.Errorf("unclosed string")
		case r == utf8.RuneError && size == 1:
			return "", len(origData) - len(data), fmt.Errorf("css: unescape string: invalid utf8 data")
		case r == quote:
			return sb.String(), len(origData) - len(data), nil
		case r == '\n':
			return "", len(origData) - len(data), fmt.Errorf("css: unescape string: newline encountered")
		case r == '\\':
			var err error
			data, err = consumeEscape(data, &sb)
			if err != nil {
				return "", len(origData) - len(data), err
			}
		default:
			sb.WriteRune(r)
		}
	}
}

func consumeEscape(data []byte, sb *strings.Builder) ([]byte, error) {
	r, size := utf8.DecodeRune(data)
	data = data[size:]
	switch {
	case r == utf8.RuneError && size == 0:
		return data, fmt.Errorf("css: unescape string: end of data in escape")
	case r == utf8.RuneError && size == 1:
		return data, fmt.Errorf("css: unescape string: invalid utf8 data")
	case isHexDigit(r):
		var digits [6]byte
		hexNumber := digits[:]
		digits[0] = byte(r)
		for i := 0; i < 5; i++ {
			r, size = utf8.DecodeRune(data)
			if !isHexDigit(r) {
				hexNumber = digits[:i+1]
				break
			}
			data = data[size:]
			digits[i+1] = byte(r)
		}
		r, size = utf8.DecodeRune(data)
		if isWhiteSpace(r) {
			data = data[size:]
		}
		runeValue, err := strconv.ParseUint(string(hexNumber), 16, 32)
		if err != nil {
			return data, err
		}
		sb.WriteRune(rune(runeValue))
	case r == '\n':
		// consume
		return data, nil
	default:
		sb.WriteRune(r)
	}
	return data, nil
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r < 'f') || (r >= 'A' && r <= 'F')
}

func isWhiteSpace(r rune) bool {
	return r == '\n' || r == '\t' || r == ' '
}
