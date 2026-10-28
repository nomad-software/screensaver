BINARY := screensaver-manager

# -s -w leaves out the symbol table and debug information, which roughly
# halves the size of the binary.
LDFLAGS := -s -w

.PHONY: build run test vet screenshot install clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./manager

run: build
	./$(BINARY)

# Every run of the tests has a timeout, so that one that hangs fails with
# the stacks of what it was waiting on, rather than waiting for ever.
TEST_TIMEOUT := 5m

test:
	go test -race -timeout $(TEST_TIMEOUT) ./manager/... ./process ./screen

vet:
	go vet ./manager/... ./process ./screen

# Draws the window, in the dark theme, with a service and programs of its own,
# into manager/screenshot.png. Run it again whenever what the window looks like
# changes.
screenshot:
	SCREENSAVER_MANAGER_SCREENSHOT=$(CURDIR)/manager/screenshot.png go test -count=1 -timeout $(TEST_TIMEOUT) -run '^TestScreenshot$$' ./manager/ui/

# Where go install puts programs: $GOBIN, or bin in $GOPATH without it.
INSTALL_DIR := $(or $(shell go env GOBIN),$(shell go env GOPATH)/bin)

# Installs the launcher, the command tool, every saver (each the file named
# for it in its folder of screen/saver) and the manager beside them.
install:
	go install cmd/launcher/screensaver-launcher.go
	go install cmd/command/screensaver-command.go
	for saver in screen/saver/*/screensaver-*.go; do go install $$saver || exit 1; done
	go build -ldflags "$(LDFLAGS)" -o $(INSTALL_DIR)/$(BINARY) ./manager

clean:
	rm -f $(BINARY)
