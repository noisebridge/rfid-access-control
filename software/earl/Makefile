# This is mostly for convenience, we don't use any of the dependency
# features of 'make'
##

VERISON ?= $(shell git log --date=short --pretty=format:'%h@%cd' -n 1 .)

all : earl test

# Note, our go version on the Pi does not understand the newer main.VERSION=xxx
# syntax
earl: *.go
	go build -ldflags "-X main.VERSION='$(VERSION)'"

test:
	go test

clean:
	rm -f earl

install:
	install init.d/earl /etc/init.d/earl
	install earl /usr/local/bin/earl
	update-rc.d earl defaults
