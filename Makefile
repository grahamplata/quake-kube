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

.PHONY: clean
clean:
	@rm -rf $(BIN_DIR)
	@rm -rf $(PWD)/assets/*

.PHONY: buildx
buildx:
	@docker buildx build . --platform=linux/amd64,linux/arm64 --progress=auto -t $(IMAGE) --push

.PHONY: network
network:
	@docker network inspect quake-net >/dev/null 2>&1 || docker network create quake-net

.PHONY: server
server: network
	@mkdir -p $(PWD)/assets
	@docker run --rm -it --name quake-server --network quake-net -p 27960:27960/udp -p 8080:8080 -v $(PWD)/config.yaml:/config.yaml -v $(PWD)/assets:/home/q3user/assets $(IMAGE) server --config /config.yaml --agree-eula --assets-dir /home/q3user/assets --content-server http://quake-content:9090 $(ARGS)

.PHONY: content
content: network
	@docker run --rm -it --name quake-content --network quake-net -p 9090:9090 -v $(PWD)/assets:/assets $(IMAGE) content --addr :9090 --assets-dir /assets

