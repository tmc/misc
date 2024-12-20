.PHONY: all test install clean

PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

all: test

test:
	shellcheck pcie-topology-analyzer.sh
	./test_suite.sh

install:
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 pcie-topology-analyzer.sh $(DESTDIR)$(BINDIR)/pcie-topology-analyzer

clean:
	rm -f *.log
	rm -rf output/

