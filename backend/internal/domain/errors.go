package domain

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrForbidden     = errors.New("forbidden")
	ErrAlreadyMember = errors.New("already a member")
	ErrInvalidInput  = errors.New("invalid input")
	ErrInvalidAmount = errors.New("invalid amount")
)
