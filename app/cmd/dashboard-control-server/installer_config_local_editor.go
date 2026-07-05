package main

import (
	"bytes"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	reWeatherProvidersList  = regexp.MustCompile(`(?s)\bweatherProviders\s*:\s*\[([^\]]*)\]`)
	reQuotedWeatherProvider = regexp.MustCompile(`"([^"]+)"`)
)

func jsString(value string) string { return strconv.Quote(strings.TrimSpace(value)) }

func configLocalBody(path string) ([]byte, error) {
	body, err := os.ReadFile(path)
	if err == nil {
		return body, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	return []byte("window.DASHBOARD_LOCAL = {\n  lat: 0,\n  lon: 0,\n  birthdays: []\n};\n"), nil
}

func setConfigLocalField(body []byte, key, value string) ([]byte, error) {
	field, found, err := configLocalField(body, key)
	if err != nil {
		return nil, err
	}
	replacement := []byte(key + ": " + value + ",")
	if found {
		out := make([]byte, 0, len(body)-field.end+field.start+len(replacement))
		out = append(out, body[:field.start]...)
		out = append(out, replacement...)
		out = append(out, body[field.end:]...)
		return out, nil
	}
	object, err := configLocalObject(body)
	if err != nil {
		return nil, err
	}
	anchor, birthdayFound, err := configLocalField(body, "birthdays")
	if err != nil {
		return nil, err
	}
	insertAt := configLocalLineStart(body, object.close)
	indent := "  "
	if birthdayFound {
		insertAt = anchor.lineStart
		indent = configLocalIndent(body, anchor.lineStart, anchor.start)
	} else if insertAt < object.close {
		indent = configLocalIndent(body, insertAt, object.close)
	}
	insert := []byte(indent + key + ": " + value + ",\n")
	if insertAt == object.close && (insertAt == 0 || body[insertAt-1] != '\n') {
		insert = append([]byte("\n"), insert...)
	}
	out := make([]byte, 0, len(body)+len(insert))
	out = append(out, body[:insertAt]...)
	out = append(out, insert...)
	out = append(out, body[insertAt:]...)
	return out, nil
}

func removeConfigLocalFields(body []byte, keys ...string) ([]byte, error) {
	for _, key := range keys {
		field, found, err := configLocalField(body, key)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		start := field.lineStart
		if len(bytes.Trim(body[start:field.start], " \t")) != 0 {
			start = field.start
		}
		end := field.end
		for end < len(body) && (body[end] == ' ' || body[end] == '\t' || body[end] == '\r') {
			end++
		}
		if end < len(body) && body[end] == '\n' {
			end++
		}
		out := make([]byte, 0, len(body)-(end-start))
		out = append(out, body[:start]...)
		out = append(out, body[end:]...)
		body = out
	}
	return body, nil
}

func bytesTrimRightSpace(value []byte) []byte {
	return []byte(strings.TrimRight(string(value), " \t\r\n"))
}
