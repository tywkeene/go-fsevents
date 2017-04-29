#!/usr/bin/env bash

go test -coverprofile cover.out
go tool cover -html=cover.out -o cover.html
$BROWSER cover.html
