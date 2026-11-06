package logs_test

import (
	"bytes"
	"context"
	"html"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/lib/logsearch"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/components/logs"
)

func TestLogLineColumnsStayLocaleNeutral(t *testing.T) {
	at := time.Date(2026, 1, 2, 15, 4, 5, 123000000, time.FixedZone("CET", 3600))
	var output bytes.Buffer
	err := logs.LogLine(logs.LogLineProps{LogLine: types.StoredLog{Timestamp: at}}).Render(context.Background(), &output)
	if err != nil {
		t.Fatalf("Log line must render: %v", err)
	}
	rendered := output.String()
	if !strings.Contains(rendered, `datetime="2026-01-02T14:04:05.123Z"`) {
		t.Fatal("Time column must carry a UTC timestamp so the browser can format the local zone")
	}
	if !regexp.MustCompile(`<time[^>]*data-log-time[^>]*>\s*</time>`).MatchString(rendered) {
		t.Fatal("Shared log HTML must leave the clock empty, because live fragments have no user locale")
	}
}

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
