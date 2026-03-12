//go:build windows

package main

import (
	"fmt"
	"io"

	"github.com/d5/tengo/v2"
)

func newTengoWriter(w io.Writer) *tengoWriter {
	return &tengoWriter{Writer: w}
}

// tengoWriter wraps an io.Writer as a tengo object with printf and println methods.
type tengoWriter struct {
	io.Writer
	tengo.ObjectImpl
}

func (w *tengoWriter) TypeName() string { return "writer" }
func (w *tengoWriter) String() string   { return "<writer>" }

func (w *tengoWriter) IndexGet(index tengo.Object) (tengo.Object, error) {
	key, ok := tengo.ToString(index)
	if !ok {
		return nil, tengo.ErrInvalidIndexType
	}
	switch key {
	case "printf":
		return w.tengoPrintf(), nil
	case "println":
		return w.tengoPrintln(), nil
	}
	return tengo.UndefinedValue, nil
}

// tengoPrintf returns a tengo function: printf(format, ...args).
// A trailing newline is appended if the formatted string does not end with one.
func (w *tengoWriter) tengoPrintf() *tengo.UserFunction {
	return &tengo.UserFunction{
		Name: "printf",
		Value: func(args ...tengo.Object) (tengo.Object, error) {
			if len(args) < 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			format, ok := tengo.ToString(args[0])
			if !ok {
				return nil, tengo.ErrInvalidArgumentType{
					Name:     "format",
					Expected: "string",
					Found:    args[0].TypeName(),
				}
			}
			fmtArgs := make([]interface{}, len(args)-1)
			for i, a := range args[1:] {
				fmtArgs[i] = tengo.ToInterface(a)
			}
			s := fmt.Sprintf(format, fmtArgs...)
			if len(s) == 0 || s[len(s)-1] != '\n' {
				s += "\n"
			}
			fmt.Fprint(w, s)
			return tengo.UndefinedValue, nil
		},
	}
}

// tengoPrintln returns a tengo function: println(...args).
// Arguments are space-separated with a trailing newline, like fmt.Println.
func (w *tengoWriter) tengoPrintln() *tengo.UserFunction {
	return &tengo.UserFunction{
		Name: "println",
		Value: func(args ...tengo.Object) (tengo.Object, error) {
			ifaces := make([]interface{}, len(args))
			for i, a := range args {
				ifaces[i] = tengo.ToInterface(a)
			}
			fmt.Fprintln(w, ifaces...)
			return tengo.UndefinedValue, nil
		},
	}
}
