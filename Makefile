SERVER_DIR ?= /opt/wake-on-wan-relay
SYSTEMD_DIR ?= /etc/systemd/system
SERVICE_FILE := wake-on-wan-relay.service
SERVER_USER ?= 1000
SERVER_GROUP ?= 1000
PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin

define SYSTEMD_UNIT
[Unit]
Description=Wake-on-WAN TLS Relay
After=network.target
StartLimitIntervalSec=60
StartLimitBurst=3

[Service]
Type=simple
WorkingDirectory=$(SERVER_DIR)
ExecStart=$(SERVER_DIR)/build/server
Restart=on-failure
RestartSec=5
User=$(SERVER_USER)
Group=$(SERVER_GROUP)

[Install]
WantedBy=multi-user.target
endef
export SYSTEMD_UNIT

.PHONY: all build build-server build-client build-keygen deploy-server install-client clean

all: build

build:
	mkdir -p build
	GOBIN=$(PWD)/build go install ./client/native ./keypair-generator ./server

build-server:
	mkdir -p build
	GOBIN=$(PWD)/build go install ./server

build-client:
	mkdir -p build
	GOBIN=$(PWD)/build go install ./client/native

build-keygen:
	mkdir -p build
	GOBIN=$(PWD)/build go install ./keypair-generator

deploy-server: build-server
	@if [ ! -f config.json ]; then \
		echo "Error: config.json not found! Copy example-config.json to config.json and configure it before deploying."; \
		exit 1; \
	fi
	sudo mkdir -p $(SERVER_DIR)/build
	sudo cp build/server $(SERVER_DIR)/build/server
	sudo chmod 755 $(SERVER_DIR)/build/server
	sudo cp config.json $(SERVER_DIR)/config.json
	@if [ -f cert.pem ]; then \
		sudo cp cert.pem $(SERVER_DIR)/cert.pem; \
	fi
	@if [ -f key.pem ]; then \
		sudo cp key.pem $(SERVER_DIR)/key.pem; \
	fi
	sudo chown -R $(SERVER_USER):$(SERVER_GROUP) $(SERVER_DIR)
	@echo "$$SYSTEMD_UNIT" | sudo tee $(SYSTEMD_DIR)/$(SERVICE_FILE) > /dev/null
	sudo chmod 644 $(SYSTEMD_DIR)/$(SERVICE_FILE)
	sudo systemctl daemon-reload && sudo systemctl enable --now wake-on-wan-relay.service

install-client: build-client
	sudo mkdir -p $(BINDIR)
	sudo cp build/client $(BINDIR)/wake-client
	sudo chmod 755 $(BINDIR)/wake-client
	sudo ln -sf wake-client $(BINDIR)/wake-on-wan-client
	@if [ ! -e $(BINDIR)/client ]; then \
		sudo ln -sf wake-client $(BINDIR)/client; \
	fi
	@echo "Client installed to $(BINDIR)/wake-client (symlinked as wake-on-wan-client)."

clean:
	rm -rf build
