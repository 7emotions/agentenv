// Package errors provides structured error types for the agentenv CLI.
//
// Errors are categorized into User, System, and Network types, each with
// a distinct exit code and a consistent user-facing format:
//
//	✗ <message>
//	  <suggestion>
//
// Use factory functions (UserError, SystemError, NetworkError) to create
// errors with actionable suggestions. Use Wrap to annotate an existing error
// while preserving its type.
package errors

import (
	"errors"
)

// ErrorType classifies the nature of an error for exit codes and messaging.
type ErrorType int

const (
	ErrorUser    ErrorType = 1 // invalid input, wrong args
	ErrorSystem  ErrorType = 2 // IO, permissions
	ErrorNetwork ErrorType = 3 // network/timeout
)

// AgentError is a structured error with a type, message, suggestion, and
// optional wrapped cause.
type AgentError struct {
	Type       ErrorType
	Message    string
	Suggestion string
	Err        error
}

// Error returns the formatted error string:
//
//	✗ <message>
//	  <suggestion>
//
// If a cause is wrapped, it is included as a second indented line.
func (e *AgentError) Error() string {
	s := "✗ " + e.Message
	if e.Err != nil {
		s += "\n  Cause: " + e.Err.Error()
	}
	if e.Suggestion != "" {
		s += "\n  " + e.Suggestion
	}
	return s
}

// Unwrap returns the wrapped cause, if any.
func (e *AgentError) Unwrap() error {
	return e.Err
}

// ExitCode returns the process exit code corresponding to the error type:
// 1 for user errors, 2 for system errors, 3 for network errors.
func (e *AgentError) ExitCode() int {
	return int(e.Type)
}

// WithCause attaches an underlying error cause and returns the receiver
// for chaining.
func (e *AgentError) WithCause(err error) *AgentError {
	e.Err = err
	return e
}

// UserError creates an ErrorUser error with the given message and suggestion.
func UserError(msg string, suggestion string) *AgentError {
	return &AgentError{Type: ErrorUser, Message: msg, Suggestion: suggestion}
}

// SystemError creates an ErrorSystem error with the given message and suggestion.
func SystemError(msg string, suggestion string) *AgentError {
	return &AgentError{Type: ErrorSystem, Message: msg, Suggestion: suggestion}
}

// NetworkError creates an ErrorNetwork error with the given message and suggestion.
func NetworkError(msg string, suggestion string) *AgentError {
	return &AgentError{Type: ErrorNetwork, Message: msg, Suggestion: suggestion}
}

// Wrap annotates an existing error with additional context, preserving its
// type if it is already an *AgentError. If err is nil, Wrap returns nil.
func Wrap(err error, msg string) *AgentError {
	if err == nil {
		return nil
	}
	var ae *AgentError
	if As(err, &ae) {
		return &AgentError{
			Type:       ae.Type,
			Message:    msg,
			Suggestion: ae.Suggestion,
			Err:        err,
		}
	}
	// Default to SystemError for bare wrapped errors
	return &AgentError{
		Type:    ErrorSystem,
		Message: msg,
		Err:     err,
	}
}

// As is a type-safe wrapper for errors.As. It finds the first error in err's
// chain that matches target and reports whether a match was found.
func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

// Is is a type-safe wrapper for errors.Is.
func Is(err, target error) bool {
	return errors.Is(err, target)
}
