# This is mostly for convenience, we don't use any of the dependency
# features of 'make'
##

all:
	go build

clean:
	rm -f earl

install:
	cp init.d/earl /etc/init.d/earl
	cp earl /usr/local/bin/earl
	update-rc.d earl defaults