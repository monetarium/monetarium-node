// Copyright (c) 2017-2022 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package sampleconfig

import (
	_ "embed"
)

// sampleMonetariumConf is a string containing the commented example config for monetarium.
//
//go:embed sample-monetarium.conf
var sampleMonetariumConf string

// sampleMonctlConf is a string containing the commented example config for
// monctl.
//
//go:embed sample-monctl.conf
var sampleMonctlConf string

// Mond returns a string containing the commented example config for monetarium.
func Mond() string {
	return sampleMonetariumConf
}

// FileContents returns a string containing the commented example config for
// mond.
//
// Deprecated: Use the [Mond] function instead.
func FileContents() string {
	return Mond()
}

// Monctl returns a string containing the commented example config for monctl.
func Monctl() string {
	return sampleMonctlConf
}

// MonctlSampleConfig is a string containing the commented example config for
// monctl.
//
// Deprecated: Use the [Monctl] function instead.
var MonctlSampleConfig = Monctl()
