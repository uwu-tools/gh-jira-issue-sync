// Copyright 2023 uwu-tools Authors
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
//
// SPDX-License-Identifier: Apache-2.0

//go:build mage

package main

import (
	"fmt"
	"os"

	"github.com/magefile/mage/sh"
	"sigs.k8s.io/release-utils/mage"
)

var (
	proj            = "gh-jira-issue-sync"
	orgPath         = "github.com/uwu-tools"
	golangciVersion = "v1.50.1"
	coverMode       = "atomic"
	coverProfile    = "unit-coverage.out"
	version, _      = sh.Output("./git-version")
	repoPath        = fmt.Sprintf("%s/%s", orgPath, proj)
	ldFlags         = fmt.Sprintf("-X %s/cmd.Version=%s", repoPath, version)
)

func init() {
	os.Setenv("GO15VENDOREXPERIMENT", "1")
	os.Setenv("CGO_ENABLED", "0")
}

// Default target to run when none is specified
// If not set, running mage will list available targets
// var Default = Build

// Create executable to bin/
func Build() error {
	return sh.RunV("go", "build", "-o", fmt.Sprintf("bin/%s", proj), "-ldflags", ldFlags, repoPath)
}

// Remove bin
func Clean() error {
	return sh.Rm("bin")
}

// Run tests
func Test() error {
	return sh.RunV("go", "test", "-v", "-covermode", coverMode, "-coverprofile", coverProfile, "./...")
}

// Run linter
func Lint() error {
	return mage.RunGolangCILint(golangciVersion, true, "-v")
}

// Run linter with auto fix enabled
func LintFix() error {
	return mage.RunGolangCILint(golangciVersion, true, "--fix")
}
