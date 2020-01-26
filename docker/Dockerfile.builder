FROM golang:1.13 AS builder
ENV GOPATH="/go" 
COPY . /go/src/github.com/tywkeene/go-fsevents
WORKDIR /go/src/github.com/tywkeene/go-fsevents
RUN go mod download
