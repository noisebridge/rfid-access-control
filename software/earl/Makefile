# This is mostly for convenience, we don't use any of the dependency
# features of 'make'
##

BRANCH    ?= $(shell git rev-parse --abbrev-ref HEAD)
BUILDDATE ?= $(shell date --iso-8601=seconds)
REVISION  ?= $(shell git rev-parse HEAD)
VERSION   ?= $(shell git log --date=short --pretty=format:'%h@%cd' -n 1 .)

all : earl test

earl: *.go
	go build -ldflags "-X main.Branch=$(BRANCH) -X main.BuildDate=$(BUILDDATE) -X main.Revision=$(REVISION) -X main.Version=$(VERSION)"

test:
	go test

clean:
	rm -f earl

install:
	install init.d/earl /etc/init.d/earl
	install earl /usr/local/bin/earl
	update-rc.d earl defaults
