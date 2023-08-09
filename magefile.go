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
