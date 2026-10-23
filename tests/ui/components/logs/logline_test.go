package logs_test

import (
	"bytes"
	"context"
	"html"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/markojerkic/svarog/internal/server/ui/components/logs"
)

func TestLogLineTextHighlightsSearchWords(t *testing.T) {
	mark := regexp.MustCompile(`<mark[^>]*>(.*?)</mark>`)
	for _, tc := range []struct {
		name, text, search string
		matches            []string
	}{
		{"no search", "ERROR connection refused", "", nil},
		{"blank search", "ERROR connection refused", " \t ", nil},
		{"no match", "connection accepted", "refused", nil},
		{"multiple occurrences and cases", "ERROR error Error", "error", []string{"ERROR", "error", "Error"}},
		{"separated terms", "Connection to server was refused", "connection refused", []string{"Connection", "refused"}},
		{"whole words", "errors error terror", "error", []string{"error"}},
		{"punctuation", "[ERROR]: connection-refused", "error & connection-refused", []string{"ERROR", "connection", "refused"}},
		{"unicode", "ŽUTA žuta greška", "žuta GREŠKA", []string{"ŽUTA", "žuta", "greška"}},
		{"duplicate terms", "error", "error ERROR", []string{"error"}},
		{"formatting", "\tERROR  \n error\t", "error", []string{"ERROR", "error"}},
		{"untrusted HTML", `<script>alert("error")</script> & <mark>ERROR</mark>`, "error", []string{"error", "ERROR"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := logs.LogLineText(tc.text, tc.search).Render(context.Background(), &output); err != nil {
				t.Fatalf("Search highlighting must render successfully: %v", err)
			}
			rendered := output.String()
			var matches []string
			for _, match := range mark.FindAllStringSubmatch(rendered, -1) {
				matches = append(matches, html.UnescapeString(match[1]))
			}
			if !reflect.DeepEqual(matches, tc.matches) {
				t.Errorf("Highlighting must mark every matching word: got %q, want %q", matches, tc.matches)
			}
			plain := html.UnescapeString(mark.ReplaceAllString(rendered, "$1"))
			if plain != tc.text {
				t.Errorf("Highlighting must preserve the exact original log text: got %q, want %q", plain, tc.text)
			}
			if strings.Contains(rendered, "<script>") || strings.Count(rendered, "<mark") != len(tc.matches) {
				t.Error("Log content must remain escaped and cannot inject HTML")
			}
		})
	}
}
