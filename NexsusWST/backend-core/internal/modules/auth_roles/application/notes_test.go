package application

import (
	"strings"
	"testing"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
)

func TestValidateNote(t *testing.T) {
	text := func(s string) *string { return &s }
	for _, tc := range []struct {
		c             domain.NoteChanges
		create, valid bool
	}{
		{domain.NoteChanges{Body: text(strings.Repeat("á", 140))}, true, true},
		{domain.NoteChanges{Body: text(strings.Repeat("á", 141))}, true, false},
		{domain.NoteChanges{Body: text(" ")}, true, false},
		{domain.NoteChanges{}, true, false},
		{domain.NoteChanges{}, false, false},
		{domain.NoteChanges{DueDateSet: true}, false, true},
		{domain.NoteChanges{DueDateSet: true, DueDate: text("2026-02-29")}, false, false},
		{domain.NoteChanges{DueDateSet: true, DueDate: text("2028-02-29")}, false, true},
	} {
		if err := validateNote(&tc.c, tc.create); (err == nil) != tc.valid {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}
