package admin

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func TestAdminPagesPreserveFragmentTargets(t *testing.T) {
	for _, tt := range []struct {
		name     string
		page     templ.Component
		bodyID   string
		dialogID string
	}{
		{"projects", ProjectsListPage(ProjectsListPageProps{Size: 20, Search: "missing"}), "projects-list-table-body", "edit-project-dialog"},
		{"users", UsersListPage(UsersListPageProps{Size: 20, Search: "missing"}), "users-list-table-body", "edit-user-dialog"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := tt.page.Render(context.Background(), &output); err != nil {
				t.Fatalf("Admin page must render successfully: %v", err)
			}
			html := output.String()
			for _, fragment := range []string{
				`id="` + tt.bodyID + `"`,
				`id="search-form"`,
				`hx-target=".table-wrapper"`,
				`hx-swap="outerHTML"`,
				`value="missing"`,
				`No ` + tt.name + ` found.`,
				`data-slot="toast-viewport"`,
			} {
				if !strings.Contains(html, fragment) {
					t.Errorf("Admin page must preserve %q", fragment)
				}
			}
			if strings.Contains(html, "Toastify") || strings.Contains(html, "toastify-js") {
				t.Error("Admin page must use the shadcn toaster, not Toastify")
			}
			tableEnd := strings.Index(html, "</table>")
			dialog := strings.Index(html, `id="`+tt.dialogID+`"`)
			if tableEnd < 0 || dialog <= tableEnd {
				t.Error("Edit dialogs must render outside the table so fragment replacement preserves valid markup")
			}
		})
	}
}
