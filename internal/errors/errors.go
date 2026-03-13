package errors

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrCodeNoSession     ErrorCode = "NO_SESSION"
	ErrCodeNotVerified   ErrorCode = "NOT_VERIFIED"
	ErrCodeAuthFailed    ErrorCode = "AUTH_FAILED"
	ErrCodeNetworkError  ErrorCode = "NETWORK_ERROR"
	ErrCodeNotFound      ErrorCode = "NOT_FOUND"
	ErrCodeInvalidInput  ErrorCode = "INVALID_INPUT"
	ErrCodeInternal      ErrorCode = "INTERNAL_ERROR"
	ErrCodeCryptoError   ErrorCode = "CRYPTO_ERROR"
	ErrCodeBridgeNotInit ErrorCode = "BRIDGE_NOT_INITIALIZED"
)

type AppError struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func New(code ErrorCode, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func Wrap(code ErrorCode, message string, cause error) *AppError {
	return &AppError{Code: code, Message: message, Cause: cause}
}

func NoSession(cause error) *AppError {
	return Wrap(ErrCodeNoSession, "no matrix session found", cause)
}

func NotVerified(cause error) *AppError {
	return Wrap(ErrCodeNotVerified, "device not verified", cause)
}

func AuthFailed(cause error) *AppError {
	return Wrap(ErrCodeAuthFailed, "authentication failed", cause)
}

func NetworkError(cause error) *AppError {
	return Wrap(ErrCodeNetworkError, "network error", cause)
}

func NotFound(message string) *AppError {
	return New(ErrCodeNotFound, message)
}

func InvalidInput(message string, cause error) *AppError {
	return Wrap(ErrCodeInvalidInput, message, cause)
}

func Internal(cause error) *AppError {
	return Wrap(ErrCodeInternal, "internal error", cause)
}

func CryptoError(cause error) *AppError {
	return Wrap(ErrCodeCryptoError, "cryptographic error", cause)
}

func BridgeNotInitialized() *AppError {
	return New(ErrCodeBridgeNotInit, "native bridge not initialized")
}

func IsCode(err error, code ErrorCode) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

func GetCode(err error) ErrorCode {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ErrCodeInternal
}
