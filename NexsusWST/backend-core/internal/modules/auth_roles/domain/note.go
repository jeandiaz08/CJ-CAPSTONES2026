package domain

import "time"

type Note struct {
	ID        int64      `json:"id"`
	Body      string     `json:"body"`
	DueDate   *string    `json:"dueDate"`
	Done      bool       `json:"done"`
	DoneAt    *time.Time `json:"doneAt"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// DueDateSet distinguishes an absent field from an explicit null.
type NoteChanges struct {
	Body       *string
	DueDate    *string
	DueDateSet bool
	Done       *bool
}
