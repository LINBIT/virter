LATESTTAG=$(shell git describe --abbrev=0 --tags | tr -d 'v')
GITHASH=$(shell git describe --abbrev=0 --always)

all: virter

.PHONY: virter
virter:
	NAME="$@"; [ -n "$(GOOS)" ] && NAME="$${NAME}-$(GOOS)"; \
	[ -n "$(GOARCH)" ] && NAME="$${NAME}-$(GOARCH)"; \
	go build -o "$$NAME" \
		-ldflags "-X 'github.com/LINBIT/virter/cmd.version=$(LATESTTAG)' \
		-X 'github.com/LINBIT/virter/cmd.builddate=$(shell LC_ALL=C date --utc)' \
		-X 'github.com/LINBIT/virter/cmd.githash=$(GITHASH)'"

.PHONY: release
release:
	make virter GOOS=linux GOARCH=amd64
