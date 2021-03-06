// Copyright 2013 Eric Myhre
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gosh

import (
	"os"
)

type commandTemplate struct {
	cmd string

	args []string

	env Env

	Opts
}

type Opts struct {
	Cwd string

	/**
	 * Can be a:
	 *   - string, in which case it will be copied in literally
	 *   - []byte, again, taken literally
	 *   - io.Reader, which will be streamed in
	 *   - bytes.Buffer, all that sort of thing, taken literally
	 *   - <-chan string, in which case that will be streamed in
	 *   - <-chan byte[], in which case that will be streamed in
	 *   - another Command, in which case that will be started with this one and its output piped into this one
	 */
	In interface{}

	/**
	 * Can be a:
	 *   - bytes.Buffer, which will be written to literally
	 *   - io.Writer, which will be written to streamingly, flushed to whenever the command flushes
	 *   - chan<- string, which will be written to streamingly, flushed to whenever a line break occurs in the output
	 *   - chan<- byte[], which will be written to streamingly, flushed to whenever the command flushes
	 *
	 * (There's nothing that's quite the equivalent of how you can give In a string, sadly; since
	 * strings are immutable in golang, you can't set Out=&str and get anywhere.)
	 */
	Out interface{}

	/**
	 * Can be all the same things Out can be, and does the same thing, but for stderr.
	 */
	Err interface{}

	/**
	 * Exit status codes that are to be considered "successful".  If not provided, [0] is the default.
	 * (If this slice is provided, zero will -not- be considered a success code unless explicitly included.)
	 */
	OkExit []int
}

var DefaultIO = Opts{
	In:  os.Stdin,
	Out: os.Stdout,
	Err: os.Stderr,
}

type Env map[string]string

type ClearEnv struct{}
