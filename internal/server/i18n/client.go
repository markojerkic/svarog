package i18n

import (
	"context"
	"encoding/json"
)

// ClientCatalog exposes only messages used by browser-side interactions.
func ClientCatalog(ctx context.Context) string {
	messages := make(map[string]string)
	for _, key := range []string{
		"common.dismiss", "common.copy_failed", "common.select_date",
		"logs.link_copied", "logs.copy_failed",
		"calendar.previous_month", "calendar.next_month",
		"calendar.choose_month", "calendar.choose_year",
		"calendar.week_number", "calendar.week", "calendar.today", "calendar.selected",
	} {
		messages[key] = T(ctx, key)
	}
	data, _ := json.Marshal(messages)
	return string(data)
}
