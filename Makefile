.PHONY: localizer
localizer:
	tinygo build -o ./bin/localizer -no-debug -panic=trap -scheduler=none -target wasi ./localizer

.PHONY: labeler
labeler:
	tinygo build -o bin/labeler -no-debug -panic=trap -scheduler=none -target wasi ./labeler

.PHONY: validator
validator:
	tinygo build -o bin/validator -no-debug -panic=trap -scheduler=none -target wasi ./validator

.PHONY: runner
runner:
	go build -o ./bin/runner ./cmd/runner
