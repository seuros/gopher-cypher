package main

import "fmt"

type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string {
	return e.msg
}

func usageErrorf(code int, format string, args ...interface{}) error {
	return &exitError{
		code: code,
		msg:  fmt.Sprintf(format, args...),
	}
}
