package email

import "errors"

type PermanentError struct {
	err error
}

func (e *PermanentError) Error() string {
	return e.err.Error()
}

func (e *PermanentError) Unwrap() error {
	return e.err
}

func NewPermanentError(err error) error {
	if err == nil {
		return nil
	}

	return &PermanentError{err: err}
}

func IsPermanentError(err error) bool {
	var permanentErr *PermanentError

	return errors.As(err, &permanentErr)
}
