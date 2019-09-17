all: handles-example loop-example

clean:
	rm -rf ./bin
	rm -f cover.*
	rm -rf ./test ./test2

loop-example:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o ./bin/example-loop -v ./examples/loop

handles-example:
	mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -o ./bin/example-handlers -v ./examples/handlers

docker-test:
	docker build -t go-fsevents:test -f Dockerfile.tests .
	docker run --rm go-fsevents:test

test-cover:
	go test -race -coverprofile cover.out ./...
	go tool cover -html=cover.out -o cover.html
	${BROWSER} cover.html
