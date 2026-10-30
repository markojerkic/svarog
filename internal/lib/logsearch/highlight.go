package logsearch

import (
	"slices"
	"strings"
)

type Fragment struct {
	Text    string
	Matched bool
}

// Highlight preserves the original text and merges overlapping matches. Rune
// positions keep Croatian letters and case mappings with different byte lengths
// from corrupting the rendered text. Fragments are plain text, never HTML.
func Highlight(text string, terms []string) []Fragment {
	if len(terms) == 0 || text == "" {
		return []Fragment{{Text: text}}
	}
	original := []rune(text)
	folded := []rune(strings.ToLower(text))
	matched := make([]bool, len(original))
	for _, term := range terms {
		needle := []rune(strings.ToLower(term))
		if len(needle) == 0 {
			continue
		}
		for start := 0; start+len(needle) <= len(folded); start++ {
			end := start + len(needle)
			if slices.Equal(folded[start:end], needle) {
				for i := start; i < end; i++ {
					matched[i] = true
				}
			}
		}
	}
	var fragments []Fragment
	for start := 0; start < len(original); {
		end := start + 1
		for end < len(original) && matched[end] == matched[start] {
			end++
		}
		fragments = append(fragments, Fragment{Text: string(original[start:end]), Matched: matched[start]})
		start = end
	}
	return fragments
}
