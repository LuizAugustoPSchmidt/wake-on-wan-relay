# Wake On WAN Relay

This project is a simple relay for Wake-On-WAN 
It assumes you have an always-on server at home

## Build

Just run:

```bash
make
```

## Configuration

As of now, all configs are housed in `config.json`. 

Since it contains private keys, it is git-ignored and must be filled before 
starting the server. To do that, just run:

```bash
cp example-config.json config.json
```

And fill it with the relevant informations. Namely, are obrigatory:
    - PublicKey
    - PrivateKey
    - MAC Address

Optional fields:
    - certPath (default: cert.pem in project root)
    - keyPath (default: key.pem in project root)
    - caCertPath (client-side path to CA or self-signed certificate; falls back to certPath if file exists)
    - certFingerprint (client-side SHA-256 fingerprint for certificate pinning)
    - serverName (client-side TLS SNI / hostname override if connecting via IP)

You can generate a key pair in the right format by running, after build:

```bash
pushd build && ./key-gen && popd
```

For now, please do not fill both `PublicKey` and `PrivateKey` in the same machine,
as that would compromise the system's safety. Instead, if you are generating the 
key pair on the server, copy the private key and send it in two halves to the 
client machine, preferably over a secure medium.

## Deployment

Deploy the server as a systemd service (copies files to `/opt/wake-on-wan-relay` and enables `wake-on-wan-relay.service`):

```bash
make deploy-server
```

Install the native client into `/usr/local/bin` (as `wake-client` / `wake-on-wan-client`):

```bash
make install-client
```

### Docker Deployment

You can build and manage the Docker container using Make:

```bash
# Build the Docker image
make docker-build

# Run the container (uses --net=host for Wake-on-LAN broadcasts)
make docker-run

# View logs / status
make docker-logs
make docker-status

# Stop / restart
make docker-stop
make docker-restart
```

Alternatively, you can run directly with `docker compose`:

```bash
docker compose up -d
```


