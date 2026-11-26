package snippet

import "testing"

// The message is what lands in a log line when a validation error is not turned
// into a 422, so it has to be stable and it has to name the fields. The order is
// sorted rather than map iteration order, which is why this asserts the whole
// string instead of a substring.
func TestValidationErrorNamesTheFieldsInOrder(t *testing.T) {
	err := &ValidationError{Fields: map[string]string{
		"title":         "is required",
		"content":       "is required",
		"expiresInDays": "must be between 1 and 365",
	}}

	const want = "invalid fields: [content expiresInDays title]"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestValidationErrorWithNoFields(t *testing.T) {
	if got := (&ValidationError{}).Error(); got != "invalid fields: []" {
		t.Errorf("Error() = %q, want %q", got, "invalid fields: []")
	}
}
