SUBDIRS := advanced basic extensions grpc http streaming

.PHONY: all $(SUBDIRS) generate

all: generate

generate: $(SUBDIRS)

$(SUBDIRS):
	$(MAKE) -C $@ generate
