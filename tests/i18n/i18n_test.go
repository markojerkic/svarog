package i18n_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	for _, test := range []struct {
		name   string
		cookie string
		header string
		want   i18n.Language
	}{
		{"default", "", "", i18n.English},
		{"croatian cookie wins", "hr", "en-US", i18n.Croatian},
		{"english cookie wins", "en", "hr-HR", i18n.English},
		{"invalid cookie uses browser", "de", "hr-HR", i18n.Croatian},
		{"regional cookie is not an explicit supported choice", "hr-HR", "en", i18n.English},
		{"croatian region", "", "hr-HR", i18n.Croatian},
		{"english region", "", "en-GB", i18n.English},
		{"case insensitive header", "", "HR-hr", i18n.Croatian},
		{"quality beats order", "", "en;q=0.5,hr;q=0.9", i18n.Croatian},
		{"same quality preserves order", "", "en,hr", i18n.English},
		{"skip unsupported preference", "", "de,hr;q=0.8,en;q=0.5", i18n.Croatian},
		{"excluded language", "", "hr;q=0,en;q=0.5", i18n.English},
		{"unrelated language", "", "de", i18n.English},
		{"related language is not croatian", "", "sr-Latn", i18n.English},
		{"wildcard does not infer english ahead of croatian", "", "*,hr;q=0.8", i18n.Croatian},
		{"region alone does not infer croatian", "", "und-HR", i18n.English},
		{"malformed header", "", "hr;q=invalid", i18n.English},
		{"cookie still wins with malformed header", "hr", "invalid header", i18n.Croatian},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, i18n.Resolve(test.cookie, test.header), "Language selection must respect the explicit choice and supported browser preferences")
		})
	}
}

func TestContextLanguagesStayIsolated(t *testing.T) {
	root := context.Background()
	require.Equal(t, i18n.English, i18n.LanguageFrom(root), "A context without a language must use English")
	require.Equal(t, "en-US", i18n.Locale(root), "The default formatting locale must be explicit")
	require.Equal(t, "Cancel", i18n.T(root, "common.cancel"), "Background renders must have an English fallback")

	hr := i18n.WithLanguage(root, i18n.Croatian)
	require.Equal(t, i18n.Croatian, i18n.LanguageFrom(hr), "The selected language must be available to renderers")
	require.Equal(t, "hr-HR", i18n.Locale(hr), "Croatian formatting must use the Croatian regional locale")
	en := i18n.WithLanguage(hr, i18n.English)
	require.Equal(t, "Cancel", i18n.T(en, "common.cancel"), "A child context may override its parent's language")
	require.Equal(t, "Odustani", i18n.T(hr, "common.cancel"), "Overriding a child must not modify its parent")
	require.Equal(t, i18n.English, i18n.LanguageFrom(i18n.WithLanguage(root, i18n.Language("de"))), "Unsupported context languages must fall back to English")

	for index := range 20 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			lang, want := i18n.English, "Cancel"
			if index%2 == 0 {
				lang, want = i18n.Croatian, "Odustani"
			}
			ctx := i18n.WithLanguage(root, lang)
			for range 20 {
				require.Equal(t, want, i18n.T(ctx, "common.cancel"), "Concurrent renders must never share a mutable language")
			}
		})
	}
}

func TestFormat(t *testing.T) {
	for _, test := range []struct {
		lang i18n.Language
		want string
	}{
		{i18n.English, "Actions for Željko <admin>"},
		{i18n.Croatian, "Radnje za korisnika Željko <admin>"},
	} {
		ctx := i18n.WithLanguage(context.Background(), test.lang)
		require.Equal(t, test.want, i18n.Format(ctx, i18n.Message{
			ID:   "users.actions_for",
			Data: map[string]any{"Username": "Željko <admin>"},
		}), "Named parameters must preserve Unicode and remain plain text for the HTML renderer to escape")
		require.Equal(t, "missing.key", i18n.T(ctx, "missing.key"), "A missing translation must return its key without panicking")
		require.Equal(t, "users.actions_for", i18n.T(ctx, "users.actions_for"), "Missing parameters must not produce partial text or <no value>")
	}
}

func TestCroatianPluralForms(t *testing.T) {
	ctx := i18n.WithLanguage(context.Background(), i18n.Croatian)
	for _, test := range []struct {
		count int
		word  string
	}{
		{0, "znakova"}, {1, "znak"}, {2, "znaka"}, {4, "znaka"}, {5, "znakova"},
		{11, "znakova"}, {12, "znakova"}, {21, "znak"}, {22, "znaka"},
	} {
		t.Run(fmt.Sprint(test.count), func(t *testing.T) {
			require.Equal(t, fmt.Sprintf("Unesite najmanje %d %s.", test.count, test.word), i18n.Format(ctx, i18n.Message{
				ID:    "validation.min_length",
				Data:  map[string]any{"Count": test.count},
				Count: &test.count,
			}), "Croatian plural forms must handle zero and teen exceptions, not only singular and plural")
		})
	}
	count := 1
	require.Equal(t, "Must be at least 1 character", i18n.Format(context.Background(), i18n.Message{
		ID: "validation.min_length", Data: map[string]any{"Count": count}, Count: &count,
	}), "English must use its own plural rules")
}

func TestEnglishFallbackForMissingCroatianMessage(t *testing.T) {
	catalog, err := i18n.NewCatalog(fstest.MapFS{
		"locales/en.json": {Data: []byte(`{"greeting":{"other":"Hello {{.Name}}"}}`)},
		"locales/hr.json": {Data: []byte(`{"other":{"other":"Druga poruka"}}`)},
	})
	require.NoError(t, err, "An incomplete translation must still load so the default language can be used")
	ctx := catalog.WithLanguage(context.Background(), i18n.Croatian)
	require.Equal(t, "Hello Željko", i18n.Format(ctx, i18n.Message{
		ID: "greeting", Data: map[string]any{"Name": "Željko"},
	}), "The fallback text must be returned even when go-i18n also reports a missing translation error")
}

func TestLoadBundleRejectsBrokenCatalogs(t *testing.T) {
	for _, test := range []struct {
		name string
		data string
	}{
		{"malformed JSON", `{"message":`},
		{"malformed template", `{"message":{"other":"Hello {{.Name"}}`},
		{"empty message", `{"message":{"other":""}}`},
		{"missing other form", `{"message":{"one":"One"}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := i18n.NewCatalog(fstest.MapFS{
				"locales/en.json": {Data: []byte(test.data)},
				"locales/hr.json": {Data: []byte(`{"message":{"other":"Poruka"}}`)},
			})
			require.Error(t, err, "Broken embedded translations must fail before serving requests")
		})
	}
}

func TestCatalogsHaveMatchingKeys(t *testing.T) {
	var keys []string
	catalogs := os.DirFS("../../internal/server/i18n")
	for _, lang := range []i18n.Language{i18n.English, i18n.Croatian} {
		data, err := fs.ReadFile(catalogs, "locales/"+string(lang)+".json")
		require.NoError(t, err, "Every supported language must have a source catalog for embedding")
		var messages map[string]map[string]string
		require.NoError(t, json.Unmarshal(data, &messages), "Catalogs must use a consistent message format")
		currentKeys := slices.Sorted(maps.Keys(messages))
		if keys != nil {
			require.Equal(t, keys, currentKeys, "Every catalog must translate the same message keys")
		}
		keys = currentKeys
	}
}
