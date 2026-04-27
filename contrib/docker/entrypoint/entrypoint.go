// Copyright (c) 2021-2023 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)

const (
	// defaultApp is the default application assumed when either no arguments
	// are specified or the first argument starts with a -.
	defaultApp = "monn"
)

// argN either returns the arguments at the provided position within the given
// args array when it exists or an empty string otherwise.
func argN(args []string, n int) string {
	if len(args) > n {
		return args[n]
	}
	return ""
}

// prepend return a new slice that consists of the provided value followed by
// the given args.
func prepend(args []string, val string) []string {
	newArgs := make([]string, 0, len(args)+1)
	newArgs = append(newArgs, val)
	newArgs = append(newArgs, args...)
	return newArgs
}

// convertsToFalse returns true if the provided string is "false", "f", or "0".
func convertsToFalse(val string) bool {
	return val == "false" || val == "f" || val == "0"
}

// fileExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func main() {
	// Name of the invoking executable.  This should typically be "entrypoint".
	exeName := filepath.Base(os.Args[0])

	// Local copy of supplied arguments without the invoking process.  This
	// allows the params to be modified independently below as needed.
	args := make([]string, len(os.Args)-1)
	copy(args, os.Args[1:])

	// Assume the provided arguments are for default app when the first
	// parameter starts with a dash.
	if arg0 := argN(args, 0); arg0 == "" || arg0[0] == '-' {
		fmt.Printf("%s: assuming arguments for %s\n", exeName, defaultApp)
		args = prepend(args, defaultApp)
	}

	// Determine monn app data directory based on environment variable.
	monetariumData := os.Getenv("MONETARIUM_DATA")
	monnAppData := filepath.Join(monetariumData, ".monn")

	// Additional setup when running in a container.
	arg0 := argN(args, 0)
	args = args[1:]
	switch arg0 {
	case "monn":
		if !convertsToFalse(os.Getenv("MON_NO_FILE_LOGGING")) {
			args = append(args, "--nofilelogging")
		}
		args = append(args, fmt.Sprintf("--appdata=%s", monnAppData))
		args = append(args, "--rpclisten=")

	case "monctl":
		// Determine monctl app data directory based on environment variable.
		rpcCert := filepath.Join(monnAppData, "rpc.cert")
		monctlAppData := filepath.Join(monetariumData, ".monctl")
		monctlConfig := filepath.Join(monctlAppData, "monctl.conf")

		// TODO: These all unconditionally override config file settings.
		// Detect if already there and don't do it?
		//
		// Prepend the arguments in case the caller wants to override them.
		args = prepend(args, fmt.Sprintf("--rpccert=%s", rpcCert))
		args = prepend(args, fmt.Sprintf("--configfile=%s", monctlConfig))

		// Change the home directory to match the data path since monctl
		// relies in it to discover the monn config file in order to extract
		// the rpc credentials.
		if !fileExists(filepath.Join(monctlAppData, "monctl.conf")) {
			os.Setenv("HOME", monetariumData)
		}
	}

	// Run the command with the given arguments while redirecting stdin, stdout,
	// and stderr to the parent process.  Also, listen for SIGTERM and forward
	// it to ensure the child process has the opportunity to perform a graceful
	// shutdown.
	cmd := exec.Command(arg0, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	go func() {
		interruptChannel := make(chan os.Signal, 1)
		signal.Notify(interruptChannel, syscall.SIGTERM)
		for sig := range interruptChannel {
			cmd.Process.Signal(sig)
		}
	}()
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cmd.ProcessState.ExitCode())
	}
}
