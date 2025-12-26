.PHONY: anydrop
anydrop:
	go build -o anydrop ./src/

.PHONY: install
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
install:
	go build -o $(DESTDIR)$(BINDIR)/anydrop ./src
