package app

import (
	"time"
	"unicode"
	"unicode/utf8"
)

type copyHit struct {
	X, Y, W, H int
	Text       string
}

type valueToken struct {
	Start int
	End   int
	Text  string
}

func isTokenRune(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	switch r {
	case '_', '.', '/', ':', '@', '%', '+', '-':
		return true
	}
	return false
}

func tokenizeValue(s string) []valueToken {
	var out []valueToken
	i := 0
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !isTokenRune(r) {
			i += size
			continue
		}
		start := i
		i += size
		for i < len(s) {
			r2, size2 := utf8.DecodeRuneInString(s[i:])
			if !isTokenRune(r2) {
				break
			}
			i += size2
		}
		out = append(out, valueToken{Start: start, End: i, Text: s[start:i]})
	}
	return out
}

func hitTest(hits []copyHit, x, y int) (string, bool) {
	for _, h := range hits {
		if y >= h.Y && y < h.Y+h.H && x >= h.X && x < h.X+h.W {
			return h.Text, true
		}
	}
	return "", false
}

type clickTracker struct {
	x, y  int
	at    time.Time
	armed bool
}

const doubleClickWindow = 500 * time.Millisecond

func (t *clickTracker) register(x, y int, now time.Time) bool {
	if t.armed && now.Sub(t.at) <= doubleClickWindow &&
		absInt(x-t.x) <= 1 && absInt(y-t.y) <= 1 {
		t.armed = false
		return true
	}
	t.x, t.y, t.at, t.armed = x, y, now, true
	return false
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
