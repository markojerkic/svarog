package i18n

import (
	"context"
	"io/fs"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

// Catalog holds validated translations and is immutable after construction.
type Catalog struct {
	bundle *goi18n.Bundle
}

// NewCatalog loads locales/en.json and locales/hr.json from source.
// Invalid catalogs are rejected; missing Croatian messages fall back to English.
func NewCatalog(source fs.FS) (*Catalog, error) {
	bundle, err := loadBundle(source)
	if err != nil {
		return nil, err
	}
	return &Catalog{bundle: bundle}, nil
}

// WithLanguage selects a language from this catalog without changing its parent context.
func (c *Catalog) WithLanguage(ctx context.Context, lang Language) context.Context {
	if lang != Croatian {
		lang = English
	}
	return context.WithValue(ctx, contextKey{}, localization{
		language:  lang,
		localizer: goi18n.NewLocalizer(c.bundle, string(lang)),
	})
}
