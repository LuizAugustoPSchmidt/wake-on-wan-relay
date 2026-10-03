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

You can generate a key pair in the right format by running, after build:

```bash
pushd build && ./key-gen && popd
```

For now, please do not fill both `PublicKey` and `PrivateKey` in the same machine,
as that would compromise the system's safety. Instead, if you are generating the 
key pair on the server, copy the private key and send it in two halves to the 
client machine, preferably over a secure medium.


