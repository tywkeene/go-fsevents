all: docker-test

clean:
	rm -rf ./bin
	rm -f cover.*
	rm -rf ./test ./test2

handlers-example:
	docker build -t fsevents-handlers:latest -f docker/Dockerfile.handlers .
	docker run --rm --name fsevents-handlers fsevents-handlers:latest

loops-example:
	docker build -t fsevents-loops:latest -f docker/Dockerfile.loop .
	docker run --rm --name fsevents-loops fsevents-loops:latest

docker-test:
	docker build -t go-fsevents:test -f docker/Dockerfile.tests .
	docker run --rm --name fsevents-test go-fsevents:test

test-cover:
	go test -race -coverprofile cover.out ./...
	go tool cover -html=cover.out -o cover.html
	${BROWSER} cover.html
