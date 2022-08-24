// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !linux
// +build !linux

package srv

import (
	"io"
	"os/exec"
)

func reexecCommandOSTweaks(cmd *exec.Cmd) {}

func userCommandOSTweaks(cmd *exec.Cmd) {}

// RunCommand reads in the command to run from the parent process (over a
// pipe) then constructs and runs the command.
func RunCommand() (errw io.Writer, code int, err error) {
	return io.Discard, 0, nil
}
