FROM golang:1.10

WORKDIR /go-fsevents
COPY . /go-fsevents/src/github.com/tywkeene/go-fsevents

RUN git clone https://github.com/golang/sys.git /go-fsevents/src/golang.org/x/sys

ENV GOPATH="/go-fsevents" 

WORKDIR /go-fsevents/src/github.com/tywkeene/go-fsevents
CMD go test -v ./...