base := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

VERSION?=0.0.1
NAME=dv-vault

all: setup

arch = $(shell uname -m)

package: clean
	go build -ldflags "-X main.SERVER_VERSION=$(VERSION)"
	mv dv-vault ./installer/dv-vault
	chmod +x ./installer/install.sh

setup: package
	makeself ./installer ./setup/$(NAME)-$(VERSION)_$(arch)_setup.sh \
		"$(NAME) $(VERSION)" ./install.sh

clean:
	rm -f ./setup/*.sh
	rm -f ./installer/dv-vault