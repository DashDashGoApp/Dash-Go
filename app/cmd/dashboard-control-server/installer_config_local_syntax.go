package main

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
)

// Installer configuration files are intentionally small, but a user may have
// formatted a field across several lines. The installer therefore edits only
// top-level assignments inside the recognized local-config object instead of
// applying line-oriented regular expressions to JavaScript source.
var reConfigLocalObject = regexp.MustCompile(`(?s)(?:window\s*\.\s*)?(?:DASHBOARD_LOCAL|DASH_CONFIG|CONFIG|config)\s*=\s*\{`)

type configLocalObjectRange struct {
	open  int
	close int
}

type configLocalFieldRange struct {
	start     int
	end       int
	lineStart int
}

func configLocalWhitespace(c byte) bool {
	switch c {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func configLocalIdentifierStart(c byte) bool {
	return c == '_' || c == '$' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func configLocalIdentifierPart(c byte) bool {
	return configLocalIdentifierStart(c) || (c >= '0' && c <= '9')
}

func configLocalLineStart(body []byte, at int) int {
	if at <= 0 {
		return 0
	}
	if line := bytes.LastIndexByte(body[:at], '\n'); line >= 0 {
		return line + 1
	}
	return 0
}

func configLocalSkipString(body []byte, at, end int) (int, error) {
	quote := body[at]
	at++
	for at < end {
		if body[at] == '\\' {
			at += 2
			continue
		}
		if body[at] == quote {
			return at + 1, nil
		}
		at++
	}
	return 0, errors.New("unterminated string in config.local.js")
}

func configLocalSkipComment(body []byte, at, end int) (int, bool, error) {
	if at+1 >= end || body[at] != '/' {
		return at, false, nil
	}
	switch body[at+1] {
	case '/':
		at += 2
		for at < end && body[at] != '\n' {
			at++
		}
		return at, true, nil
	case '*':
		close := bytes.Index(body[at+2:end], []byte("*/"))
		if close < 0 {
			return 0, true, errors.New("unterminated comment in config.local.js")
		}
		return at + 2 + close + 2, true, nil
	default:
		return at, false, nil
	}
}

func configLocalSkipSpaceAndComments(body []byte, at, end int) (int, error) {
	for {
		for at < end && configLocalWhitespace(body[at]) {
			at++
		}
		next, comment, err := configLocalSkipComment(body, at, end)
		if err != nil {
			return 0, err
		}
		if !comment {
			return at, nil
		}
		at = next
	}
}

func configLocalBalancedClose(body []byte, open int) (int, error) {
	if open < 0 || open >= len(body) || body[open] != '{' {
		return 0, errors.New("config.local.js object does not start with an opening brace")
	}
	depth := 0
	for at := open; at < len(body); at++ {
		switch body[at] {
		case '\'', '"', '`':
			next, err := configLocalSkipString(body, at, len(body))
			if err != nil {
				return 0, err
			}
			at = next - 1
		case '/':
			next, comment, err := configLocalSkipComment(body, at, len(body))
			if err != nil {
				return 0, err
			}
			if comment {
				at = next - 1
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return at, nil
			}
		}
	}
	return 0, errors.New("unterminated local configuration object")
}

func configLocalObject(body []byte) (configLocalObjectRange, error) {
	match := reConfigLocalObject.FindIndex(body)
	if match == nil {
		return configLocalObjectRange{}, errors.New("config.local.js does not contain a recognized local configuration object")
	}
	open := bytes.LastIndexByte(body[:match[1]], '{')
	close, err := configLocalBalancedClose(body, open)
	if err != nil {
		return configLocalObjectRange{}, err
	}
	return configLocalObjectRange{open: open, close: close}, nil
}

func configLocalReadPropertyName(body []byte, at, end int) (string, int, error) {
	if at >= end {
		return "", 0, errors.New("missing property name in config.local.js")
	}
	if body[at] == '\'' || body[at] == '"' {
		quote := body[at]
		start := at + 1
		next, err := configLocalSkipString(body, at, end)
		if err != nil {
			return "", 0, err
		}
		value := string(body[start : next-1])
		if bytes.IndexByte([]byte(value), '\\') >= 0 {
			return "", 0, errors.New("escaped config.local.js property names are not supported by the installer editor")
		}
		_ = quote
		return value, next, nil
	}
	if !configLocalIdentifierStart(body[at]) {
		return "", 0, fmt.Errorf("unsupported top-level syntax near %q", string(body[at:min(at+24, end)]))
	}
	start := at
	at++
	for at < end && configLocalIdentifierPart(body[at]) {
		at++
	}
	return string(body[start:at]), at, nil
}

func configLocalValueEnd(body []byte, at, objectClose int) (int, int, error) {
	braces, brackets, parens := 0, 0, 0
	for at < objectClose {
		switch body[at] {
		case '\'', '"', '`':
			next, err := configLocalSkipString(body, at, objectClose)
			if err != nil {
				return 0, 0, err
			}
			at = next
			continue
		case '/':
			next, comment, err := configLocalSkipComment(body, at, objectClose)
			if err != nil {
				return 0, 0, err
			}
			if comment {
				at = next
				continue
			}
		case '{':
			braces++
		case '}':
			if braces > 0 {
				braces--
			}
		case '[':
			brackets++
		case ']':
			if brackets > 0 {
				brackets--
			}
		case '(':
			parens++
		case ')':
			if parens > 0 {
				parens--
			}
		case ',':
			if braces == 0 && brackets == 0 && parens == 0 {
				end := at
				for end > 0 && configLocalWhitespace(body[end-1]) {
					end--
				}
				return end, at, nil
			}
		}
		at++
	}
	end := objectClose
	for end > 0 && configLocalWhitespace(body[end-1]) {
		end--
	}
	if braces != 0 || brackets != 0 || parens != 0 {
		return 0, 0, errors.New("unbalanced value in config.local.js")
	}
	return end, -1, nil
}

func configLocalField(body []byte, key string) (configLocalFieldRange, bool, error) {
	object, err := configLocalObject(body)
	if err != nil {
		return configLocalFieldRange{}, false, err
	}
	at := object.open + 1
	for at < object.close {
		at, err = configLocalSkipSpaceAndComments(body, at, object.close)
		if err != nil {
			return configLocalFieldRange{}, false, err
		}
		if at >= object.close {
			break
		}
		if body[at] == ',' {
			at++
			continue
		}
		start := at
		name, next, err := configLocalReadPropertyName(body, at, object.close)
		if err != nil {
			return configLocalFieldRange{}, false, err
		}
		at, err = configLocalSkipSpaceAndComments(body, next, object.close)
		if err != nil {
			return configLocalFieldRange{}, false, err
		}
		if at >= object.close || body[at] != ':' {
			return configLocalFieldRange{}, false, fmt.Errorf("property %q is missing a colon in config.local.js", name)
		}
		at, err = configLocalSkipSpaceAndComments(body, at+1, object.close)
		if err != nil {
			return configLocalFieldRange{}, false, err
		}
		valueEnd, comma, err := configLocalValueEnd(body, at, object.close)
		if err != nil {
			return configLocalFieldRange{}, false, err
		}
		fieldEnd := valueEnd
		if comma >= 0 {
			fieldEnd = comma + 1
		}
		field := configLocalFieldRange{start: start, end: fieldEnd, lineStart: configLocalLineStart(body, start)}
		if name == key {
			return field, true, nil
		}
		if comma < 0 {
			break
		}
		at = comma + 1
	}
	return configLocalFieldRange{}, false, nil
}

func configLocalIndent(body []byte, lineStart, at int) string {
	if lineStart < at {
		candidate := body[lineStart:at]
		if len(bytes.Trim(candidate, " \t")) == 0 {
			return string(candidate)
		}
	}
	return "  "
}
