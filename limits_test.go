package main

import (
	"errors"
	"testing"
)

func TestRejectExcessTaskTimeout(t *testing.T) {
	for _, seconds := range []int{121, 600, 900} {
		_, _, err := validate(t.Context(), Input{Task: "Document API", Files: []string{"/missing.go"}, AcceptanceCriteria: []string{"Comments only"}, TimeoutSec: seconds})
		var typed *serverError
		if !errors.As(err, &typed) || typed.Code != "invalid_input" {
			t.Fatalf("timeout %d was not rejected before filesystem access: %v", seconds, err)
		}
	}
}
