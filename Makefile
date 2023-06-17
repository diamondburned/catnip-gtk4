.PHONY: build run

GOFILES = $(shell find . -name '*.go')
BLUEPRINTS = $(shell find . -name '*.blueprint')
BLUEPRINT_UIS = $(patsubst %.blueprint, %.blueprint.ui, $(BLUEPRINTS))

build: catnip-gtk4

run: catnip-gtk4
	./catnip-gtk4

catnip-gtk4: $(GOFILES) $(BLUEPRINT_UIS)
	go build -v

%.blueprint.ui: %.blueprint
	blueprint-compiler compile --output $@ $<
