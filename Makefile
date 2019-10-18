SHELL=/bin/bash

EXE = go-search

all: $(EXE)

go-search:
	@echo "building $@ ..."
	$(MAKE) -s -f make.inc s=static

clean:
	rm -f $(EXE)

