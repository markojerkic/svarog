package validation

// Violation identifies a rule and any parameters needed to describe it.
type Violation struct {
	Key     string
	Count   int
	Options []string // Message IDs for the allowed values, in display order.
}

// Error describes invalid input independently of the request language.
type Error struct {
	Key    string
	Fields map[string]Violation
}

func (e *Error) Error() string {
	return e.Key
}
