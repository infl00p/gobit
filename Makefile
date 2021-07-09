SHELL := /bin/bash

TARGET := $(shell echo $${PWD##*/})
.DEFAULT_GOAL: $(TARGET)

VERSION := 0.2
BUILD := `git rev-parse HEAD`
CGO_ENABLED=0

#LDFLAGS=-ldflags "-extldflags -static -v -X=main.Version=$(VERSION) -X=main.Build=$(BUILD)"
LDFLAGS=-ldflags "-v -X=main.Version=$(VERSION) -X=main.Build=$(BUILD)"

.PHONY: all build clean run

all: clean build strip

$(TARGET): $(SRC)
	@go build $(LDFLAGS) -o $(TARGET)

build: $(TARGET)
	@true

clean:
	@rm -f $(TARGET)

strip:
	@strip $(TARGET)

run:
	@go run .
