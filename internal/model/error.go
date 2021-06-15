package model

import (
	"bytes"
	"fmt"
	"strings"
)

type validationError struct {
	message   string
	subErrors []error
}

func newValidationError(message string) *validationError {
	return &validationError{message: message}
}

func (v *validationError) orNil() error {
	if len(v.subErrors) == 0 {
		return nil
	}
	return v
}

func (v *validationError) Error() string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(v.errorPrefix(nil, true))
}

func (v *validationError) errorPrefix(prefix []rune, last bool) string {
	sep := ":\n"
	if len(v.subErrors) == 0 {
		sep = ""
	}
	errMsg := bytes.NewBufferString(fmt.Sprintf("%s%s%s", string(prefix), v.message, sep))
	for i, serr := range v.subErrors {
		subPrefix := prefix
		if len(subPrefix) >= 4 {
			if last {
				subPrefix = append(subPrefix[0:len(subPrefix)-4], []rune("    ")...)
			} else {
				subPrefix = append(subPrefix[0:len(subPrefix)-4], []rune("│   ")...)
			}
		}
		subLast := i == len(v.subErrors)-1
		if subLast {
			subPrefix = append(subPrefix, []rune("└── ")...)
		} else {
			subPrefix = append(subPrefix, []rune("├── ")...)
		}
		if verr, ok := serr.(*validationError); ok {
			errMsg.WriteString(verr.errorPrefix(subPrefix, subLast))
		} else {
			errMsg.WriteString(fmt.Sprintf("%s%s\n", string(subPrefix), serr))
		}
	}
	return errMsg.String()
}
