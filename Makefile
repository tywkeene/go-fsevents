all: docker-test

clean:
	rm -rf ./bin
	rm -f cover.*
	rm -rf ./test ./test2

docker-builder:
	docker build -t fsevents-docker-builder:latest -f docker/Dockerfile.builder .

handlers-example: docker-builder
	docker build -t fsevents-handlers:latest -f docker/Dockerfile.handlers .
	docker run --name fsevents-handlers fsevents-handlers:latest

loops-example: docker-builder
	docker build -t fsevents-loops:latest -f docker/Dockerfile.loop .
	docker run --name fsevents-loops fsevents-loops:latest

docker-test: docker-builder
	docker build -t go-fsevents:test -f docker/Dockerfile.tests .
	docker run --rm --name fsevents-test go-fsevents:test

docker-bench: docker-builder
	docker build -t go-fsevents:bench -f docker/Dockerfile.bench .
	docker run --rm --name fsevents-bench --volume ${PWD}/bench-data:/bench-data go-fsevents:bench

test-cover:
	go test -race -coverprofile cover.out ./...
	go tool cover -html=cover.out -o cover.html
	${BROWSER} cover.html
