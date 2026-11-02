package logsearch

import (
	"errors"
	"strings"
	"unicode"
)

var ErrTooShort = errors.New("search needs a term with three consecutive letters or digits")

// Query contains literal, case-insensitive substrings, all of which must match.
// PrimaryPattern is an explicit ILIKE predicate so PostgreSQL can use pg_trgm;
// RemainingPatterns are additional filters and may contain shorter terms.
type Query struct {
	Terms             []string
	PrimaryPattern    string
	RemainingPatterns []string
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func Parse(input string) (Query, error) {
	query := Query{RemainingPatterns: []string{}}
	seen := make(map[string]bool)
	primary, longest := 0, 0
	for _, word := range strings.Fields(input) {
		word = strings.ToLower(word)
		if seen[word] {
			continue
		}
		seen[word] = true
		query.Terms = append(query.Terms, word)
		if length := longestAlphanumericRun(word); length > longest {
			primary, longest = len(query.Terms)-1, length
		}
	}
	if len(query.Terms) == 0 {
		return query, nil
	}
	if longest < 3 {
		return Query{}, ErrTooShort
	}
	for i, term := range query.Terms {
		pattern := "%" + likeEscaper.Replace(term) + "%"
		if i == primary {
			query.PrimaryPattern = pattern
		} else {
			query.RemainingPatterns = append(query.RemainingPatterns, pattern)
		}
	}
	return query, nil
}

func longestAlphanumericRun(text string) int {
	longest, current := 0, 0
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current++
			longest = max(longest, current)
		} else {
			current = 0
		}
	}
	return longest
}
