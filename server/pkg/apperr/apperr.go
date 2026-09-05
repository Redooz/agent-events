package apperr

import (
	"errors"
	"fmt"
)

type Kind string

const (
	KindNotFound     Kind = "not_found"
	KindInvalid      Kind = "invalid"
	KindUnauthorized Kind = "unauthorized"
	KindForbidden    Kind = "forbidden"
	KindTooMany      Kind = "too_many"
	KindInternal     Kind = "internal"
)

type Error struct {
	Kind    Kind
	Message string
	Details map[string]string
	Err     error
}

func New(kind Kind, message string) *Error {
	return &Error{Kind: kind, Message: message}
}

func NotFound(message string) *Error { return New(KindNotFound, message) }

func Invalid(message string) *Error { return New(KindInvalid, message) }

func Unauthorized(message string) *Error { return New(KindUnauthorized, message) }

func Forbidden(message string) *Error { return New(KindForbidden, message) }

func TooMany(message string) *Error { return New(KindTooMany, message) }

func Wrap(err error, message string) *Error {
	return &Error{Kind: KindInternal, Message: message, Err: err}
}

func (e *Error) Error() string {
	if e.Err == nil {
		return e.Message
	}

	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) WithDetails(details map[string]string) *Error {
	e.Details = details

	return e
}

func From(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}

	return nil, false
}

func KindOf(err error) Kind {
	if e, ok := From(err); ok {
		return e.Kind
	}

	return KindInternal
}

func MessageOf(err error) string {
	if e, ok := From(err); ok {
		return e.Message
	}

	return err.Error()
}

func DetailsOf(err error) map[string]string {
	if e, ok := From(err); ok {
		return e.Details
	}

	return nil
}
