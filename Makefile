.PHONY: build install

build:
	go build -o ksdl .

install: build
	mv ksdl /usr/local/bin/
