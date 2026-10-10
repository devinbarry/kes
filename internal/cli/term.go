// Copyright 2024 - MinIO, Inc. All rights reserved.
// Use of this source code is governed by the AGPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"os"

	"golang.org/x/term"
)

var isTerm = term.IsTerminal(int(os.Stdout.Fd())) || term.IsTerminal(int(os.Stderr.Fd()))

// IsTerminal reports whether stdout is a terminal.
func IsTerminal() bool { return isTerm }
