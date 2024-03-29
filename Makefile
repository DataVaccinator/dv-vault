base := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

VERSION?=0.0.1
NAME=dv-vault

all: setup

arch = $(shell uname -m)

package: clean
	go test
	go build -ldflags "-X main.SERVER_VERSION=$(VERSION)" -o vaccinator
	mv vaccinator ./installer/vaccinator
	chmod +x ./installer/install.sh

setup: package
	makeself ./installer ./setup/$(NAME)-$(VERSION)_$(arch)_setup.sh \
		"$(NAME) $(VERSION)" ./install.sh

clean:
	rm -f ./setup/*.sh
	rm -f ./installer/vaccinator