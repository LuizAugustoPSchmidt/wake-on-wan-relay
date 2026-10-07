SERVER_DIR ?= /opt/wake-on-wan-relay
SYSTEMD_DIR ?= /etc/systemd/system
SERVICE_FILE := wake-on-wan-relay.service
SERVER_USER ?= 1000
SERVER_GROUP ?= 1000
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

# Version extraction from Git
GIT_TAG ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.3")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_DIRTY ?= $(shell git diff --quiet 2>/dev/null || echo "-dirty")
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null || echo "$(GIT_TAG)-dev+$(GIT_COMMIT)$(GIT_DIRTY)")
LDFLAGS := -X 'wake-on-wan-relay/lib.VERSION=$(VERSION)'

DOCKER_IMAGE ?= wake-on-wan-relay
DOCKER_TAG ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "latest")
DOCKER_CONTAINER ?= wake-on-wan-relay
DOCKER_BIN ?= /usr/bin/docker

define SYSTEMD_UNIT
[Unit]
Description=Wake-on-WAN TLS Relay (Docker)
After=network.target docker.service
Requires=docker.service
StartLimitIntervalSec=60
StartLimitBurst=3

[Service]
Type=simple
Restart=on-failure
RestartSec=5
ExecStartPre=-$(DOCKER_BIN) stop $(DOCKER_CONTAINER)
ExecStartPre=-$(DOCKER_BIN) rm $(DOCKER_CONTAINER)
ExecStart=$(DOCKER_BIN) run --name $(DOCKER_CONTAINER) --net=host --read-only --cap-drop=ALL --security-opt=no-new-privileges:true -v $(SERVER_DIR)/config.json:/app/config.json:ro -v $(SERVER_DIR)/cert.pem:/app/cert.pem:ro -v $(SERVER_DIR)/key.pem:/app/key.pem:ro $(DOCKER_IMAGE):$(DOCKER_TAG)
ExecStop=-$(DOCKER_BIN) stop $(DOCKER_CONTAINER)

[Install]
WantedBy=multi-user.target
endef
export SYSTEMD_UNIT

.PHONY: all build build-server build-client build-keygen deploy-server deploy-server-native install-client clean \
        docker-build docker-run docker-stop docker-restart docker-logs docker-status \
        version tag-patch tag-minor tag-major

all: build

build:
	mkdir -p build
	GOBIN=$(PWD)/build go install -ldflags="$(LDFLAGS)" ./client/native ./keypair-generator ./server

build-server:
	mkdir -p build
	GOBIN=$(PWD)/build go install -ldflags="$(LDFLAGS)" ./server

build-client:
	mkdir -p build
	GOBIN=$(PWD)/build go install -ldflags="$(LDFLAGS)" ./client/native

build-keygen:
	mkdir -p build
	GOBIN=$(PWD)/build go install -ldflags="$(LDFLAGS)" ./keypair-generator

deploy-server: docker-build
	@if [ ! -f config.json ]; then \
		echo "Error: config.json not found! Copy example-config.json to config.json and configure it before deploying."; \
		exit 1; \
	fi
	sudo mkdir -p $(SERVER_DIR)
	sudo chmod 700 $(SERVER_DIR)
	sudo cp config.json $(SERVER_DIR)/config.json
	sudo chmod 600 $(SERVER_DIR)/config.json
	@if [ -f cert.pem ]; then \
		sudo cp cert.pem $(SERVER_DIR)/cert.pem; \
		sudo chmod 644 $(SERVER_DIR)/cert.pem; \
	elif [ ! -f $(SERVER_DIR)/cert.pem ]; then \
		sudo touch $(SERVER_DIR)/cert.pem; \
		sudo chmod 644 $(SERVER_DIR)/cert.pem; \
	fi
	@if [ -f key.pem ]; then \
		sudo cp key.pem $(SERVER_DIR)/key.pem; \
		sudo chmod 600 $(SERVER_DIR)/key.pem; \
	elif [ ! -f $(SERVER_DIR)/key.pem ]; then \
		sudo touch $(SERVER_DIR)/key.pem; \
		sudo chmod 600 $(SERVER_DIR)/key.pem; \
	fi
	sudo chown -R $(SERVER_USER):$(SERVER_GROUP) $(SERVER_DIR)
	@echo "$$SYSTEMD_UNIT" | sudo tee $(SYSTEMD_DIR)/$(SERVICE_FILE) > /dev/null
	sudo chmod 644 $(SYSTEMD_DIR)/$(SERVICE_FILE)
	sudo systemctl daemon-reload && sudo systemctl enable wake-on-wan-relay.service && sudo systemctl restart wake-on-wan-relay.service

deploy-server-native: build-server
	@if [ ! -f config.json ]; then \
		echo "Error: config.json not found! Copy example-config.json to config.json and configure it before deploying."; \
		exit 1; \
	fi
	sudo mkdir -p $(SERVER_DIR)/build
	sudo chmod 700 $(SERVER_DIR)
	sudo cp build/server $(SERVER_DIR)/build/server
	sudo chmod 755 $(SERVER_DIR)/build/server
	sudo cp config.json $(SERVER_DIR)/config.json
	sudo chmod 600 $(SERVER_DIR)/config.json
	@if [ -f cert.pem ]; then \
		sudo cp cert.pem $(SERVER_DIR)/cert.pem; \
		sudo chmod 644 $(SERVER_DIR)/cert.pem; \
	fi
	@if [ -f key.pem ]; then \
		sudo cp key.pem $(SERVER_DIR)/key.pem; \
		sudo chmod 600 $(SERVER_DIR)/key.pem; \
	fi
	sudo chown -R $(SERVER_USER):$(SERVER_GROUP) $(SERVER_DIR)
	sudo systemctl daemon-reload && sudo systemctl enable wake-on-wan-relay.service && sudo systemctl restart wake-on-wan-relay.service

install-client: build-client
	sudo mkdir -p $(BINDIR)
	sudo cp build/client $(BINDIR)/wake-client
	sudo chmod 755 $(BINDIR)/wake-client
	sudo ln -sf wake-client $(BINDIR)/wake-on-wan-client
	@if [ ! -e $(BINDIR)/client ]; then \
		sudo ln -sf wake-client $(BINDIR)/client; \
	fi
	@echo "Client installed to $(BINDIR)/wake-client (symlinked as wake-on-wan-client)."

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(DOCKER_IMAGE):$(DOCKER_TAG) -t $(DOCKER_IMAGE):latest .

docker-run:
	@if [ ! -f config.json ]; then \
		echo "Error: config.json not found! Copy example-config.json to config.json and configure it first."; \
		exit 1; \
	fi
	@RUN_ARGS="-d --name $(DOCKER_CONTAINER) --restart unless-stopped --net host --read-only --cap-drop=ALL --security-opt=no-new-privileges:true -v $(PWD)/config.json:/app/config.json:ro"; \
	if [ -f cert.pem ]; then RUN_ARGS="$$RUN_ARGS -v $(PWD)/cert.pem:/app/cert.pem:ro"; fi; \
	if [ -f key.pem ]; then RUN_ARGS="$$RUN_ARGS -v $(PWD)/key.pem:/app/key.pem:ro"; fi; \
	echo "Starting container $(DOCKER_CONTAINER)..."; \
	docker run $$RUN_ARGS $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-stop:
	-docker stop $(DOCKER_CONTAINER)
	-docker rm $(DOCKER_CONTAINER)

docker-restart: docker-stop docker-run

docker-logs:
	docker logs -f $(DOCKER_CONTAINER)

docker-status:
	docker ps -a --filter name=$(DOCKER_CONTAINER)

version:
	@echo "Build Version:  $(VERSION)"
	@echo "Latest Git Tag: $(GIT_TAG)"

tag-patch:
	@TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.3"); \
	RAW="$${TAG#v}"; IFS="." read -r MA MI PA <<< "$$RAW"; \
	NEXT="v$$MA.$$MI.$$((PA+1))"; \
	echo "Tagging $$TAG -> $$NEXT"; \
	git tag -a "$$NEXT" -m "Release $$NEXT"; \
	echo "Created tag $$NEXT."

tag-minor:
	@TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.3"); \
	RAW="$${TAG#v}"; IFS="." read -r MA MI PA <<< "$$RAW"; \
	NEXT="v$$MA.$$((MI+1)).0"; \
	echo "Tagging $$TAG -> $$NEXT"; \
	git tag -a "$$NEXT" -m "Release $$NEXT"; \
	echo "Created tag $$NEXT."

tag-major:
	@TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.3"); \
	RAW="$${TAG#v}"; IFS="." read -r MA MI PA <<< "$$RAW"; \
	NEXT="v$$((MA+1)).0.0"; \
	echo "Tagging $$TAG -> $$NEXT"; \
	git tag -a "$$NEXT" -m "Release $$NEXT"; \
	echo "Created tag $$NEXT."

clean:
	rm -rf build
