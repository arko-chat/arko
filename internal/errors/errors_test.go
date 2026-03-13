package errors

import (
	"errors"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AppError
		expected string
	}{
		{
			name:     "error without cause",
			err:      New(ErrCodeNotFound, "resource not found"),
			expected: "[NOT_FOUND] resource not found",
		},
		{
			name:     "error with cause",
			err:      Wrap(ErrCodeNetworkError, "connection failed", errors.New("timeout")),
			expected: "[NETWORK_ERROR] connection failed: timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := Wrap(ErrCodeInternal, "wrapped", cause)

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestConstructorFunctions(t *testing.T) {
	cause := errors.New("cause")

	tests := []struct {
		name     string
		err      *AppError
		expected ErrorCode
	}{
		{"NoSession", NoSession(cause), ErrCodeNoSession},
		{"NotVerified", NotVerified(cause), ErrCodeNotVerified},
		{"AuthFailed", AuthFailed(cause), ErrCodeAuthFailed},
		{"NetworkError", NetworkError(cause), ErrCodeNetworkError},
		{"NotFound", NotFound("not found"), ErrCodeNotFound},
		{"InvalidInput", InvalidInput("bad input", cause), ErrCodeInvalidInput},
		{"Internal", Internal(cause), ErrCodeInternal},
		{"CryptoError", CryptoError(cause), ErrCodeCryptoError},
		{"BridgeNotInitialized", BridgeNotInitialized(), ErrCodeBridgeNotInit},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.expected {
				t.Errorf("Code = %s, want %s", tt.err.Code, tt.expected)
			}
		})
	}
}

func TestIsCode(t *testing.T) {
	err := New(ErrCodeNotFound, "not found")

	if !IsCode(err, ErrCodeNotFound) {
		t.Error("IsCode should return true for matching code")
	}

	if IsCode(err, ErrCodeInternal) {
		t.Error("IsCode should return false for non-matching code")
	}

	if IsCode(errors.New("standard error"), ErrCodeNotFound) {
		t.Error("IsCode should return false for non-AppError")
	}
}

func TestGetCode(t *testing.T) {
	err := New(ErrCodeAuthFailed, "auth failed")
	if code := GetCode(err); code != ErrCodeAuthFailed {
		t.Errorf("GetCode() = %s, want %s", code, ErrCodeAuthFailed)
	}

	standardErr := errors.New("standard error")
	if code := GetCode(standardErr); code != ErrCodeInternal {
		t.Errorf("GetCode() for non-AppError = %s, want %s", code, ErrCodeInternal)
	}
}
