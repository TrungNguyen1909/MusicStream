TOPTARGETS := all clean

SUBDIRS := $(wildcard plugins/*/.)

$(TOPTARGETS): $(SUBDIRS)
$(SUBDIRS):
	make -C $@

.PHONY: $(TOPTARGETS) $(SUBDIRS)