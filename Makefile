.PHONY: localizer
localizer:
	tinygo build -o bin/localizer -no-debug -panic=trap -scheduler=none -target wasi ./localizer

.PHONY: labeler
labeler:
	tinygo build -o bin/labeler -no-debug -panic=trap -scheduler=none -target wasi ./labeler

run: build
	./bin/runner ./bin/localizer

build: localizer
	go build -o ./bin/runner ./cmd/runner
