package i18n

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"testing"
	"testing/fstest"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	for _, test := range []struct {
		name   string
		cookie string
		header string
		want   Language
	}{
		{"default", "", "", English},
		{"croatian cookie wins", "hr", "en-US", Croatian},
		{"english cookie wins", "en", "hr-HR", English},
		{"invalid cookie uses browser", "de", "hr-HR", Croatian},
		{"regional cookie is not an explicit supported choice", "hr-HR", "en", English},
		{"croatian region", "", "hr-HR", Croatian},
		{"english region", "", "en-GB", English},
		{"case insensitive header", "", "HR-hr", Croatian},
		{"quality beats order", "", "en;q=0.5,hr;q=0.9", Croatian},
		{"same quality preserves order", "", "en,hr", English},
		{"skip unsupported preference", "", "de,hr;q=0.8,en;q=0.5", Croatian},
		{"excluded language", "", "hr;q=0,en;q=0.5", English},
		{"unrelated language", "", "de", English},
		{"related language is not croatian", "", "sr-Latn", English},
		{"wildcard does not infer english ahead of croatian", "", "*,hr;q=0.8", Croatian},
		{"region alone does not infer croatian", "", "und-HR", English},
		{"malformed header", "", "hr;q=invalid", English},
		{"cookie still wins with malformed header", "hr", "invalid header", Croatian},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, Resolve(test.cookie, test.header), "Language selection must respect the explicit choice and supported browser preferences")
		})
	}
}

func TestContextLanguagesStayIsolated(t *testing.T) {
	root := context.Background()
	require.Equal(t, English, LanguageFrom(root), "A context without a language must use English")
	require.Equal(t, "en-US", Locale(root), "The default formatting locale must be explicit")
	require.Equal(t, "Cancel", T(root, "common.cancel"), "Background renders must have an English fallback")

	hr := WithLanguage(root, Croatian)
	require.Equal(t, Croatian, LanguageFrom(hr), "The selected language must be available to renderers")
	require.Equal(t, "hr-HR", Locale(hr), "Croatian formatting must use the Croatian regional locale")
	en := WithLanguage(hr, English)
	require.Equal(t, "Cancel", T(en, "common.cancel"), "A child context may override its parent's language")
	require.Equal(t, "Odustani", T(hr, "common.cancel"), "Overriding a child must not modify its parent")
	require.Equal(t, English, LanguageFrom(WithLanguage(root, Language("de"))), "Unsupported context languages must fall back to English")

	for index := range 20 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			lang, want := English, "Cancel"
			if index%2 == 0 {
				lang, want = Croatian, "Odustani"
			}
			ctx := WithLanguage(root, lang)
			for range 20 {
				require.Equal(t, want, T(ctx, "common.cancel"), "Concurrent renders must never share a mutable language")
			}
		})
	}
}

func TestFormat(t *testing.T) {
	for _, test := range []struct {
		lang Language
		want string
	}{
		{English, "Actions for Željko <admin>"},
		{Croatian, "Radnje za korisnika Željko <admin>"},
	} {
		ctx := WithLanguage(context.Background(), test.lang)
		require.Equal(t, test.want, Format(ctx, Message{
			ID:   "users.actions_for",
			Data: map[string]any{"Username": "Željko <admin>"},
		}), "Named parameters must preserve Unicode and remain plain text for the HTML renderer to escape")
		require.Equal(t, "missing.key", T(ctx, "missing.key"), "A missing translation must return its key without panicking")
		require.Equal(t, "users.actions_for", T(ctx, "users.actions_for"), "Missing parameters must not produce partial text or <no value>")
	}
}

func TestCroatianPluralForms(t *testing.T) {
	ctx := WithLanguage(context.Background(), Croatian)
	for _, test := range []struct {
		count int
		word  string
	}{
		{0, "znakova"}, {1, "znak"}, {2, "znaka"}, {4, "znaka"}, {5, "znakova"},
		{11, "znakova"}, {12, "znakova"}, {21, "znak"}, {22, "znaka"},
	} {
		t.Run(fmt.Sprint(test.count), func(t *testing.T) {
			require.Equal(t, fmt.Sprintf("Unesite najmanje %d %s.", test.count, test.word), Format(ctx, Message{
				ID:    "validation.min_length",
				Data:  map[string]any{"Count": test.count},
				Count: &test.count,
			}), "Croatian plural forms must handle zero and teen exceptions, not only singular and plural")
		})
	}
	count := 1
	require.Equal(t, "Must be at least 1 character", Format(context.Background(), Message{
		ID: "validation.min_length", Data: map[string]any{"Count": count}, Count: &count,
	}), "English must use its own plural rules")
}

func TestEnglishFallbackForMissingCroatianMessage(t *testing.T) {
	b, err := loadBundle(fstest.MapFS{
		"locales/en.json": {Data: []byte(`{"greeting":{"other":"Hello {{.Name}}"}}`)},
		"locales/hr.json": {Data: []byte(`{"other":{"other":"Druga poruka"}}`)},
	})
	require.NoError(t, err, "An incomplete translation must still load so the default language can be used")
	ctx := context.WithValue(context.Background(), contextKey{}, localization{
		language: Croatian, localizer: goi18n.NewLocalizer(b, "hr"),
	})
	require.Equal(t, "Hello Željko", Format(ctx, Message{
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
			_, err := loadBundle(fstest.MapFS{
				"locales/en.json": {Data: []byte(test.data)},
				"locales/hr.json": {Data: []byte(`{"message":{"other":"Poruka"}}`)},
			})
			require.Error(t, err, "Broken embedded translations must fail before serving requests")
		})
	}
}

func TestCatalogsHaveMatchingKeys(t *testing.T) {
	var keys []string
	for _, lang := range []Language{English, Croatian} {
		data, err := catalogs.ReadFile("locales/" + string(lang) + ".json")
		require.NoError(t, err, "Every supported language must have an embedded catalog")
		var messages map[string]map[string]string
		require.NoError(t, json.Unmarshal(data, &messages), "Catalogs must use a consistent message format")
		currentKeys := slices.Sorted(maps.Keys(messages))
		if keys != nil {
			require.Equal(t, keys, currentKeys, "Every catalog must translate the same message keys")
		}
		keys = currentKeys
	}
}
