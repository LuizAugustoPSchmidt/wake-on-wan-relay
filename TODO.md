# To Do WakeOnWan Relay

  - [x] Write Dockerfile
  - [x] Write Systemd service
  - [ ] Ensure Behavior
  - [ ] Test the application
  - [x] Write Android Client
  - [x] Write `flake.nix` (package + NixOS module)
  - [x] Add `devShells` block to `flake.nix`

## Client Features
  - [ ] Add status check to native client
  - [ ] Add status check to Android client

## Security & Architectural Enhancements
  - [ ] Split `config.json` into `server-config.json` and `client-config.json`
  - [ ] Add rate limiting & CPU / DoS exhaustion mitigation (IP-based throttle)
  - [ ] Add Wake-on-LAN broadcast cooldown (prevent UDP flood storms)
  - [ ] Cap banner reads in `CheckSSH` with `io.LimitReader` (prevent memory exhaustion)
  - [ ] Set `MaxHeaderBytes` (8KB) and enforce empty body checks on GET requests
