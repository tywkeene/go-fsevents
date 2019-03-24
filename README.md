# go-fsevents
[![GoDoc](https://godoc.org/github.com/tywkeene/go-fsevents?status.svg)](https://godoc.org/github.com/tywkeene/go-fsevents)
[![Build Status](https://travis-ci.org/tywkeene/go-fsevents.svg?branch=master)](https://travis-ci.org/tywkeene/go-fsevents)
[![codecov.io Code Coverage](https://img.shields.io/codecov/c/github/tywkeene/go-fsevents.svg?maxAge=2592000)](https://codecov.io/github/tywkeene/go-fsevents?branch=master)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)  



Recursive filesystem event watcher using inotify in golang

go-fsevents provides functions necessary for monitoring filesystem events on linux systems using the inotify interface.

Unlike other inotify packages, go-fsevents provides a recursive watcher, allowing the monitoring of directory trees easily.

## UNSTABLE

The package is currently unstable, and as such should not be used in any production environment.

Many changes, additions and breaking refactors will take place between now and the stable 1.0.0 release.

You have been warned.

## Features
- Single directory event monitoring
- Recursive directory tree event monitoring
- EventHandle interface to allow for clean and concise handling of events
- Access to the underlying raw inotify event through the [unix](https://godoc.org/golang.org/x/sys/unix) package
- Predefined event translations. No need to fuss with raw inotify flags.
- Concurrency safe

## Quickstart

See [example/example.go](https://github.com/tywkeene/go-fsevents/blob/master/example/example.go) to get an idea of how to use go-fsevents
