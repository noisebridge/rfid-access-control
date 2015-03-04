# This is mostly for convenience, we don't use any of the dependency
# features of 'make'
##

all : earl test

earl: *.go
	go build -ldflags "-X main.VERSION '`git log --date=short --pretty=format:'%h@%cd' -n 1`'"

test:
	go test

clean:
	rm -f earl

install:
	install init.d/earl /etc/init.d/earl
	install earl /usr/local/bin/earl
	update-rc.d earl defaults
