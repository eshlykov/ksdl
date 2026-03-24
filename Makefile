.PHONY: build install

build:
	go build -o ksdl .

install: build
	# cp+rm instead of mv: cp writes a new inode, so reinstalling while the binary
	# is running does not fail with "Text file busy" (mv would replace in-place).
	cp ksdl $(shell go env GOPATH)/bin/ && rm ksdl
