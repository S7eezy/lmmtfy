package engine

import (
	"strings"
	"unicode"
)

// Files drift: a column gets renamed, gains a space or an accent. When a
// source column is missing, the header that looks most like it is
// suggested.

// suggest returns the candidate that looks most like name, or "". A
// candidate looks like name when they are equal once case, accents, spaces
// and punctuation are ignored, or else close in edit distance: at most a
// quarter of the length, at least 1.
func suggest(name string, candidates []string) string {
	key := looseName(name)
	if key == "" {
		return ""
	}
	best, bestDist := "", max(1, len([]rune(key))/4)+1
	for _, c := range candidates {
		k := looseName(c)
		if k == "" {
			continue
		}
		if k == key {
			return c
		}
		if d := distance(key, k, bestDist); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best
}

// folds are accented letters and their plain spelling.
var folds = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'æ': "ae",
	'ç': "c", 'è': "e", 'é': "e", 'ê': "e", 'ë': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ñ': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'œ': "oe",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ý': "y", 'ÿ': "y", 'ß': "ss",
}

// looseName is a header name without case, accents, spaces or punctuation.
func looseName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case folds[r] != "":
			b.WriteString(folds[r])
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// distance is the Levenshtein distance between a and b, or limit when it
// is limit or more.
func distance(a, b string, limit int) int {
	ra, rb := []rune(a), []rune(b)
	if d := len(ra) - len(rb); d >= limit || -d >= limit {
		return limit
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		low := cur[0]
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			low = min(low, cur[j])
		}
		if low >= limit {
			return limit
		}
		prev, cur = cur, prev
	}
	return min(prev[len(rb)], limit)
}
