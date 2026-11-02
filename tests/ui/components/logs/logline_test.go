package logs_test

import (
	"bytes"
	"context"
	"html"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/markojerkic/svarog/internal/lib/logsearch"
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
		{"substrings", "errors error terror", "error", []string{"error", "error", "error"}},
		{"punctuation", "[ERROR]: connection-refused", "error connection-refused", []string{"ERROR", "connection-refused"}},
		{"literal symbols", `100% foo_bar C:\logs\app`, `100% foo_bar C:\logs\app`, []string{"100%", "foo_bar", `C:\logs\app`}},
		{"overlapping terms", "Exception", "cept eptio", []string{"ceptio"}},
		{"overlapping occurrences", "banana", "ana", []string{"anana"}},
		{"unicode substring", "Čekanje ŽUTA", "čEK žut", []string{"Ček", "ŽUT"}},
		{"unicode", "ŽUTA žuta greška", "žuta GREŠKA", []string{"ŽUTA", "žuta", "greška"}},
		{"duplicate terms", "error", "error ERROR", []string{"error"}},
		{"formatting", "\tERROR  \n error\t", "error", []string{"ERROR", "error"}},
		{"untrusted HTML", `<script>alert("error")</script> & <mark>ERROR</mark>`, "error", []string{"error", "ERROR"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query, err := logsearch.Parse(tc.search)
			if err != nil {
				t.Fatalf("Valid search terms must parse: %v", err)
			}
			var output bytes.Buffer
			if err := logs.LogLineText(tc.text, query).Render(context.Background(), &output); err != nil {
				t.Fatalf("Search highlighting must render successfully: %v", err)
			}
			rendered := output.String()
			var matches []string
			for _, match := range mark.FindAllStringSubmatch(rendered, -1) {
				matches = append(matches, html.UnescapeString(match[1]))
			}
			if !reflect.DeepEqual(matches, tc.matches) {
				t.Errorf("Highlighting must mark every matching substring: got %q, want %q", matches, tc.matches)
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
