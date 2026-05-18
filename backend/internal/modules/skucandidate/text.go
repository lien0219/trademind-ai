package skucandidate

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	reWhitespaceSKU = regexp.MustCompile(`[\s\-_]+`)
	rePunctTok      = regexp.MustCompile(`[[:punct:]]+`)
	reSpaces        = regexp.MustCompile(`\s+`)
)

// normalizeSKUCode: trim, lowercase, remove ASCII spaces/dash/underscore,
// normalize common fullwidth variants to halfwidth-ish form for comparison.
func normalizeSKUCode(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'Ａ' && r <= 'Ｚ' {
			r = r - 'Ａ' + 'a'
		} else if r >= 'ａ' && r <= 'ｚ' {
			r = r - 'ａ' + 'a'
		} else if r >= '０' && r <= '９' {
			r = r - '０' + '0'
		} else if r == '－' || r == '﹣' || r == '―' || r == '–' {
			r = '-'
		} else if r == '＿' {
			r = '_'
		} else if r == '　' {
			continue
		}
		b.WriteRune(r)
	}
	s = strings.TrimSpace(b.String())
	s = strings.ToLower(s)
	s = reWhitespaceSKU.ReplaceAllString(s, "")
	return s
}

func tokenize(text string) []string {
	text = strings.TrimSpace(strings.ToLower(text))
	text = rePunctTok.ReplaceAllString(text, " ")
	text = reSpaces.ReplaceAllString(text, " ")
	if text == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	appendTok := func(t string) {
		t = strings.TrimSpace(t)
		if t == "" {
			return
		}
		if _, ok := seen[t]; ok {
			return
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	for _, field := range strings.Fields(text) {
		var lat strings.Builder
		flush := func() {
			if lat.Len() > 0 {
				appendTok(lat.String())
				lat.Reset()
			}
		}
		for _, r := range field {
			if unicode.Is(unicode.Han, r) {
				flush()
				appendTok(string(r))
			} else {
				lat.WriteRune(r)
			}
		}
		flush()
	}
	return out
}

func tokenSet(text string) map[string]struct{} {
	ts := tokenize(text)
	m := map[string]struct{}{}
	for _, t := range ts {
		m[t] = struct{}{}
	}
	return m
}

// tokenJaccard returns [0,1] overlap of token sets derived from string a and b.
func tokenJaccard(a, b string) float64 {
	A := tokenSet(a)
	B := tokenSet(b)
	if len(A) == 0 || len(B) == 0 {
		return 0
	}
	var inter int
	for t := range A {
		if _, ok := B[t]; ok {
			inter++
		}
	}
	union := len(A) + len(B) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func bestProductTitleOverlap(orderText string, title, original, ai string) float64 {
	best := 0.0
	for _, cand := range []string{title, original, ai} {
		j := tokenJaccard(orderText, cand)
		if j > best {
			best = j
		}
	}
	return best
}

// titleScoreFromJaccard maps Jaccard to [40..60] for suggestions.
func titleScoreFromJaccard(j float64) int {
	if j <= 0 {
		return 0
	}
	raw := int(40 + j*20)
	if raw > 60 {
		return 60
	}
	return raw
}
