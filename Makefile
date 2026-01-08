BIN_DIR ?= bin
LDFLAGS := -s -w
GOFLAGS = -gcflags "all=-trimpath=$(PWD)" -asmflags "all=-trimpath=$(PWD)"
GO_BUILD_ENV_VARS := GO111MODULE=on CGO_ENABLED=0
VERSION ?= latest
IMAGE   ?= docker.io/grahamplata/quake:$(VERSION)

q3: gen
	@$(GO_BUILD_ENV_VARS) go build -o $(BIN_DIR)/q3 $(GOFLAGS) -ldflags '$(LDFLAGS)' ./cli

gen: ## Generate and embed templates
	@go run tools/genstatic.go public public

.PHONY: test
test:
	@go test -v ./internal/...

.PHONY: build
build:
	@docker build . --no-cache --force-rm --build-arg GOPROXY --build-arg GOSUMDB -t $(IMAGE)

.PHONY: buildx
buildx:
	@docker buildx build . --platform=linux/amd64,linux/arm64 --progress=auto -t $(IMAGE) --push

.PHONY: run
run:
	@mkdir -p $(PWD)/assets
	@docker run --rm -it -p 27960:27960/udp -p 8080:8080 -v $(PWD)/config.yaml:/config.yaml -v $(PWD)/assets:/home/q3user/assets $(IMAGE) server --config /config.yaml --agree-eula --assets-dir /home/q3user/assets --no-assets-download
