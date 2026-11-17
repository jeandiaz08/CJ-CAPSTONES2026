package application

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
)

func validateNote(c *domain.NoteChanges, create bool) error {
	if create && c.Body == nil {
		return fmt.Errorf("%w: indica el texto de la nota", domain.ErrInvalid)
	}
	if c.Body != nil {
		v := strings.TrimSpace(*c.Body)
		c.Body = &v
		if n := utf8.RuneCountInString(v); n < 1 || n > 140 {
			return fmt.Errorf("%w: texto de 1 a 140 caracteres", domain.ErrInvalid)
		}
	}
	if c.DueDateSet && c.DueDate != nil {
		if _, err := time.Parse("2006-01-02", *c.DueDate); err != nil {
			return fmt.Errorf("%w: fecha inválida (AAAA-MM-DD)", domain.ErrInvalid)
		}
	}
	if !create && c.Body == nil && !c.DueDateSet && c.Done == nil {
		return fmt.Errorf("%w: indica al menos un cambio", domain.ErrInvalid)
	}
	return nil
}

func (s *Service) ListNotes(ctx context.Context, userID int64) ([]domain.Note, error) {
	return s.users.ListNotes(ctx, userID)
}

func (s *Service) CreateNote(ctx context.Context, userID int64, c domain.NoteChanges) (domain.Note, error) {
	if err := validateNote(&c, true); err != nil {
		return domain.Note{}, err
	}
	return s.users.CreateNote(ctx, userID, c)
}

func (s *Service) UpdateNote(ctx context.Context, userID, noteID int64, c domain.NoteChanges) (domain.Note, error) {
	if err := validateNote(&c, false); err != nil {
		return domain.Note{}, err
	}
	return s.users.UpdateNote(ctx, userID, noteID, c)
}

func (s *Service) DeleteNote(ctx context.Context, userID, noteID int64) error {
	return s.users.DeleteNote(ctx, userID, noteID)
}
