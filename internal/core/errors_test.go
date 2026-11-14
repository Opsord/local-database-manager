package core

import (
	"errors"
	"strings"
	"testing"
)

func TestAppError_FormattingAndUnwrap(t *testing.T) {
	cause := errors.New("address already in use")
	appErr := NewAppError(ErrCodePortInUse, "Port 5432 is occupied", "Change PORT in instance config [Enter -> e]", cause)

	if appErr.Code != ErrCodePortInUse {
		t.Errorf("got code %q, want %q", appErr.Code, ErrCodePortInUse)
	}

	if !errors.Is(appErr, cause) {
		t.Errorf("errors.Is should unwrap underlying cause")
	}

	msg := appErr.Error()
	if !strings.Contains(msg, "Port 5432 is occupied") || !strings.Contains(msg, "Change PORT") {
		t.Errorf("error message missing suggestion, got: %s", msg)
	}

	var extracted *AppError
	if !errors.As(appErr, &extracted) {
		t.Fatalf("errors.As should match *AppError")
	}
	if extracted.Suggestion == "" {
		t.Errorf("extracted Suggestion is empty")
	}
}
