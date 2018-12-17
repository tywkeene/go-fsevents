test:
	docker build -t go-fsevents .
	docker run -it --rm go-fsevents sh scripts/test/cover.sh