# How to Contribute

This projects is [Apache 2.0 licensed](/LICENSE) and accept contributions via
pull requests. This document outlines some of the conventions on development
workflow, commit message formatting, contact points and other resources to make
it easier to get your contribution accepted.

## Developer Certificate of Origin

By contributing to this project, you agree to the Developer Certificate of
Origin (DCO). This document was created by the Linux Kernel community and is a
simple statement that you, as a contributor, have the legal right to make the
contribution. See the [DCO](/DCO) file for details.

## Support

The project currently uses [GitHub issues](https://github.com/uwu-tools/gh-jira-issue-sync/issues)
to provide support.

Please avoid emailing maintainers directly as we actively review the issues and
pull requests contained in this repository.

## Reporting a security vulnerability

See [SECURITY.md](/SECURITY.md).

## Getting Started

- Fork the repository on GitHub
- Read the [README](/README.md) for build and test instructions
- Play with the project, submit bugs, submit patches!

### Contribution Flow

This is a rough outline of what a contributor's workflow looks like:

- Create a topic/feature branch from where you want to base your work (usually
  `main`)
- Make commits of logical units
- Make sure your commit messages are in the proper format (see below)
- Push your changes to a topic/feature branch in your fork of the repository
- Make sure the tests pass, and add any new tests as appropriate.
- Submit a pull request to the original repository

Thanks for your contributions!

### Coding Style

This project has linters enabled, which run as part of our presubmit checks.

While we don't currently have our own style guide, we do attempt to adhere to
good examples in other Golang projects, like the Kubernetes SIG Release
[code contribution expectations](https://git.k8s.io/sig-release/CONTRIBUTING.md#coding-style)
and [coding style](https://git.k8s.io/sig-release/CONTRIBUTING.md#coding-style).

Please follow them when working on your contributions.

### Format of the Commit Message

We follow a rough convention for commit messages that is designed to answer two
questions: what changed and why. The subject line should feature the what and
the body of the commit should describe the why.

```
scripts: add the test-cluster command

this uses tmux to setup a test cluster that you can easily kill and
start for debugging.
```

The format can be described more formally as follows:

```
<subsystem>: <what changed>
<BLANK LINE>
<why this change was made>
<BLANK LINE>
<footer>
```

The first line is the subject and should be no longer than 70 characters, the
second line is always blank, and other lines should be wrapped at 80
characters.
This allows the message to be easier to read on GitHub as well as in various
git tools.
