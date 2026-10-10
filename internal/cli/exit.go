// Copyright 2024 - MinIO, Inc. All rights reserved.
// Use of this source code is governed by the AGPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"fmt"
	"os"

	tui "charm.land/lipgloss/v2"
)

// Exit prints args as error message and aborts with exit code 1.
func Exit(args ...any) {
	FG := tui.Color("#ac0000")
	s := tui.NewStyle().Foreground(FG).Render("Error: ")

	tui.Fprintln(os.Stderr, s+fmt.Sprint(args...))
	os.Exit(1)
}

// Exitf formats args as error message and aborts with exit code 1.
func Exitf(format string, args ...any) {
	FG := tui.Color("#ac0000")
	s := tui.NewStyle().Foreground(FG).Render("Error: ")

	tui.Fprintln(os.Stderr, s+fmt.Sprintf(format, args...))
	os.Exit(1)
}

// Assert calls Exit if the statement is false.
func Assert(statement bool, args ...any) {
	if !statement {
		Exit(args...)
	}
}

// Assertf calls Exitf if the statement is false.
func Assertf(statement bool, format string, args ...any) {
	if !statement {
		Exitf(format, args...)
	}
}
