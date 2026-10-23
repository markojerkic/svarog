package i18n

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/nicksnyder/go-i18n/v2/i18n/template"
	"golang.org/x/text/language"
)

type Language string

const (
	English  Language = "en"
	Croatian Language = "hr"
)

// Message describes a translation before a language is selected.
type Message struct {
	ID    string
	Data  map[string]any
	Count *int // nil means no plural count; zero is a valid count.
}

type contextKey struct{}

type localization struct {
	language  Language
	localizer *goi18n.Localizer
}

//go:embed locales/*.json
var catalogs embed.FS

var (
	textParser       = &template.TextParser{Option: "missingkey=error"}
	defaultCatalog   = mustLoadCatalog(catalogs)
	defaultLocalizer = goi18n.NewLocalizer(defaultCatalog.bundle, string(English))
)

func mustLoadCatalog(source fs.FS) *Catalog {
	catalog, err := NewCatalog(source)
	if err != nil {
		panic(err)
	}
	return catalog
}

func loadBundle(source fs.FS) (*goi18n.Bundle, error) {
	b := goi18n.NewBundle(language.English)
	for _, lang := range []Language{English, Croatian} {
		path := "locales/" + string(lang) + ".json"
		file, err := b.LoadMessageFileFS(source, path)
		if err != nil {
			return nil, fmt.Errorf("load translations %s: %w", path, err)
		}
		for _, message := range file.Messages {
			if message.Other == "" {
				return nil, fmt.Errorf("translation %s in %s needs an other form", message.ID, path)
			}
			// go-i18n parses templates lazily; reject broken catalogs at startup.
			for _, text := range []string{message.Zero, message.One, message.Two, message.Few, message.Many, message.Other} {
				if text == "" {
					continue
				}
				if _, err := textParser.Parse(text, message.LeftDelim, message.RightDelim); err != nil {
					return nil, fmt.Errorf("parse translation %s in %s: %w", message.ID, path, err)
				}
			}
		}
	}
	return b, nil
}

// Resolve prefers an explicit supported cookie over supported browser languages.
func Resolve(cookieValue, acceptLanguage string) Language {
	if cookieValue == string(English) || cookieValue == string(Croatian) {
		return Language(cookieValue)
	}
	tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil {
		return English
	}
	for _, tag := range tags {
		base, confidence := tag.Base()
		if confidence != language.Exact {
			continue
		}
		switch base.String() {
		case string(English):
			return English
		case string(Croatian):
			return Croatian
		}
	}
	return English
}

func WithLanguage(ctx context.Context, lang Language) context.Context {
	return defaultCatalog.WithLanguage(ctx, lang)
}

func current(ctx context.Context) localization {
	if value, ok := ctx.Value(contextKey{}).(localization); ok {
		return value
	}
	return localization{language: English, localizer: defaultLocalizer}
}

func LanguageFrom(ctx context.Context) Language {
	return current(ctx).language
}

func Locale(ctx context.Context) string {
	if LanguageFrom(ctx) == Croatian {
		return "hr-HR"
	}
	return "en-US"
}

func T(ctx context.Context, id string) string {
	return Format(ctx, Message{ID: id})
}

func Format(ctx context.Context, message Message) string {
	value := current(ctx)
	config := &goi18n.LocalizeConfig{
		MessageID:      message.ID,
		TemplateData:   message.Data,
		TemplateParser: textParser,
	}
	if message.Count != nil {
		config.PluralCount = *message.Count
	}
	text, err := value.localizer.Localize(config)
	if err != nil {
		slog.WarnContext(ctx, "Translation failed", "language", value.language, "messageID", message.ID, "error", err)
	}
	if text == "" {
		return message.ID
	}
	return text
}
