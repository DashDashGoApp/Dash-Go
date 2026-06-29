package maps

import (
	"regexp"
	"strings"
)

// Map query cleanup is kept separate from provider calls so cache keys and
// fallback candidates stay deterministic and easy to test without network I/O.
var mapStreetAbbrevReplacements = []struct {
	re  *regexp.Regexp
	sub string
}{
	{regexp.MustCompile(`(?i)\bN\b`), "North"}, {regexp.MustCompile(`(?i)\bS\b`), "South"},
	{regexp.MustCompile(`(?i)\bE\b`), "East"}, {regexp.MustCompile(`(?i)\bW\b`), "West"},
	{regexp.MustCompile(`(?i)\bNE\b`), "Northeast"}, {regexp.MustCompile(`(?i)\bNW\b`), "Northwest"},
	{regexp.MustCompile(`(?i)\bSE\b`), "Southeast"}, {regexp.MustCompile(`(?i)\bSW\b`), "Southwest"},
	{regexp.MustCompile(`(?i)\bRd\.?\b`), "Road"}, {regexp.MustCompile(`(?i)\bSt\.?\b`), "Street"},
	{regexp.MustCompile(`(?i)\bAve\.?\b`), "Avenue"}, {regexp.MustCompile(`(?i)\bBlvd\.?\b`), "Boulevard"},
	{regexp.MustCompile(`(?i)\bDr\.?\b`), "Drive"}, {regexp.MustCompile(`(?i)\bLn\.?\b`), "Lane"},
	{regexp.MustCompile(`(?i)\bCt\.?\b`), "Court"}, {regexp.MustCompile(`(?i)\bHwy\.?\b`), "Highway"},
	{regexp.MustCompile(`(?i)\bPkwy\.?\b`), "Parkway"},
}

func cleanLocationPiece(v string) string {
	v = strings.Trim(v, " ,;\t\r\n")
	v = reWhitespace.ReplaceAllString(v, " ")
	return v
}

func expandUSStreetAbbrevs(v string) string {
	v = cleanLocationPiece(v)
	for _, replacement := range mapStreetAbbrevReplacements {
		v = replacement.re.ReplaceAllString(v, replacement.sub)
	}
	return cleanLocationPiece(v)
}

func looksLikeUSAddress(v string) bool {
	v = cleanLocationPiece(v)
	return reUSPostal.MatchString(v) || reUSCountry.MatchString(v)
}

func looksLikeStreetAddress(v string) bool {
	v = cleanLocationPiece(v)
	if !reStreetNumber.MatchString(v) {
		return false
	}
	return reStreetSuffix.MatchString(v)
}

func eventMapQueryVariants(q string) []string {
	raw := strings.ReplaceAll(q, "\r", "\n")
	lines := []string{}
	for line := range strings.SplitSeq(raw, "\n") {
		if c := cleanLocationPiece(line); c != "" {
			lines = append(lines, c)
		}
	}
	flat := cleanLocationPiece(raw)
	if len(flat) > 220 {
		flat = flat[:220]
	}
	out := []string{}
	add := func(v string) {
		v = cleanLocationPiece(v)
		if len(v) > 220 {
			v = v[:220]
		}
		if len(v) < 3 {
			return
		}
		for _, old := range out {
			if strings.EqualFold(old, v) {
				return
			}
		}
		out = append(out, v)
	}
	add(flat)
	if len(lines) > 1 {
		add(strings.Join(lines, ", "))
		add(strings.Join(lines[1:], ", "))
	}
	pieces := []string{}
	if len(lines) > 0 {
		for _, line := range lines {
			for part := range strings.SplitSeq(line, ",") {
				if c := cleanLocationPiece(part); c != "" {
					pieces = append(pieces, c)
				}
			}
		}
	} else if flat != "" {
		for part := range strings.SplitSeq(flat, ",") {
			if c := cleanLocationPiece(part); c != "" {
				pieces = append(pieces, c)
			}
		}
	}
	for i, piece := range pieces {
		if reStreetNumber.MatchString(piece) {
			add(strings.Join(pieces[i:], ", "))
			break
		}
	}
	for _, v := range append([]string{}, out...) {
		add(reUSCountrySuffix.ReplaceAllString(v, ""))
	}
	for _, v := range append([]string{}, out...) {
		if looksLikeStreetAddress(v) || looksLikeUSAddress(v) {
			add(expandUSStreetAbbrevs(v))
		}
	}
	if len(out) > 10 {
		return out[:10]
	}
	return out
}
