BOT_NAME  := orgbot
CLI_NAME  := orgctl
BUILD_DIR := target
BIN_DIR   := $(BUILD_DIR)/bin
BINARIES  := $(BIN_DIR)/linux/amd64/$(BOT_NAME) \
						 $(BIN_DIR)/darwin/amd64/$(BOT_NAME) \
						 $(BIN_DIR)/linux/amd64/$(CLI_NAME) \
						 $(BIN_DIR)/darwin/amd64/$(CLI_NAME) \

GO_SRC    := $(shell find . -type f -name '*.go' -not -path "./vendor/*")
SH_SRC    := $(shell find . -type f -name '*.sh' -not -path "./vendor/*")

VERSION    := $(shell cat VERSION)
GIT_COMMIT ?= $(shell git rev-parse HEAD 2> /dev/null || echo dunno)
BUILD_TIME := $(shell date --utc --rfc-3339=seconds 2> /dev/null | sed -e 's/ /T/')

NO_COLOR    := \033[0m
OK_COLOR    := \033[32;01m
ERROR_COLOR := \033[31;01m
WARN_COLOR  := \033[33;01m

export GO111MODULE=on

all: $(BINARIES)

linux-all: target/bin/linux/amd64/orgbot
linux-all: target/bin/linux/amd64/orgctl

darwin-all: target/bin/darwin/amd64/orgbot
darwin-all: target/bin/darwin/amd64/orgctl

.PHONY: test
test:
	@echo "\n$(OK_COLOR)====> Running tests$(NO_COLOR)"
	go test -mod=vendor ./...

.PHONY: clean
clean:
	@echo "\n$(OK_COLOR)====> Cleaning$(NO_COLOR)"
	go clean ./... && rm -rf ./$(BUILD_DIR)

.PHONY: shellcheck
shellcheck:
	@echo "\n$(OK_COLOR)====> Running shellcheck$(NO_COLOR)"
	shellcheck -x ".buildkite/lib/common.sh" $(SH_SRC)

.PHONY: lint
lint:
	@echo "\n$(OK_COLOR)====> Running shfmt$(NO_COLOR)"
	shfmt -i 2 -ci -sr -bn -d $(SH_SRC)

.PHONY: snyk
snyk:
	@echo "\n$(OK_COLOR)====> Running snyk$(NO_COLOR)"
	snyk test --org=paved-road
	snyk monitor --org=paved-road


$(BINARIES): splitted=$(subst /, ,$@)
$(BINARIES): os=$(word 3, $(splitted))
$(BINARIES): arch=$(word 4, $(splitted))
$(BINARIES): cmd=$(basename $(word 5, $(splitted)))
$(BINARIES): $(GO_SRC)
	@echo "\n$(OK_COLOR)====> Building $@$(NO_COLOR)"
	GOOS=$(os) GOARCH=$(arch) CGO_ENABLED=0 \
		go build -mod=vendor -a -ldflags=all=" \
		  -X github.com/SEEK-Jobs/orgbot/pkg/build.Name=$(cmd) \
			-X github.com/SEEK-Jobs/orgbot/pkg/build.Version=$(VERSION) \
			-X github.com/SEEK-Jobs/orgbot/pkg/build.GitCommit=$(GIT_COMMIT) \
			-X github.com/SEEK-Jobs/orgbot/pkg/build.BuildTime=$(BUILD_TIME) \
			-X github.com/SEEK-Jobs/orgbot/pkg/build.OperatingSystem=$(os) \
			-X github.com/SEEK-Jobs/orgbot/pkg/build.Architecture=$(arch)" \
		  -o $@ cmd/$(cmd)/*.go

.PHONY: mockgen
mockgen:
	@echo "\n$(OK_COLOR)====> Generating mocks $$dir$(NO_COLOR)"
	@rm -rf $(BUILD_DIR)/mockgen
	@mkdir -p $(BUILD_DIR)/mockgen
	@cp \
		pkg/orgbot/org.go \
		pkg/orgbot/common.go \
		pkg/orgbot/github.go \
		pkg/orgbot/rules.go \
		$(BUILD_DIR)/mockgen
	@for f in github rules; do \
		mockgen \
			-source=$(BUILD_DIR)/mockgen/$${f}.go \
			-destination=$(BUILD_DIR)/mockgen/$${f}_mock.go \
			-package=orgbot \
			-self_package=github.com/SEEK-Jobs/orgbot/$(BUILD_DIR)/mockgen; \
	done
	@rm -f pkg/orgbot/mocks_gen.go
	@cat $(BUILD_DIR)/mockgen/github_mock.go > pkg/orgbot/mocks_gen.go
	@cat $(BUILD_DIR)/mockgen/rules_mock.go | sed '/package/d' | sed '/import (/,/)/d' >> pkg/orgbot/mocks_gen.go
