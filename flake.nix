{
  description = "Flake for Wake-On-WAN-Relay configuration";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
  };
  outputs =
    { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
      version = "1.2.0-${self.shortRev or self.dirtyShortRev or "unknown"}";
    in
    {
      formatter.${system} = pkgs.nixfmt;
      packages.${system} = {
        default = pkgs.buildGo127Module {
          pname = "wake-on-wan-relay";
          inherit version;
          src = self;
          vendorHash = null;
          ldflags = [
            "-s"
            "-w"
            "-X wake-on-wan-relay/lib.VERSION=${version}"
          ];
        };
      };

      devShells.${system} = {
        default = pkgs.mkShell {
          packages = with pkgs; [
            go_1_27
            gopls
            gnumake
            nixfmt
          ];
          shellHook = ''
            			echo "Started wake-on-wan-relay Dev Shell"
            		'';
        };
      };

      nixosModules.default =
        {
          config,
          lib,
          pkgs,
          ...
        }:
        let
          cfg = config.services.wake-on-wan-relay;
        in
        {
          options.services.wake-on-wan-relay = {
            enable = lib.mkEnableOption "Wake On WAN Relay Server";
            configFile = lib.mkOption {
              type = lib.types.str;
              default = "/opt/wake-on-wan-relay/config.json";
              description = "Runtime path to the config.json file";
            };
          };
          config = lib.mkIf cfg.enable {
            systemd.services.wake-on-wan-relay = {
              description = "Wake-on-WAN TLS Relay";
              after = [ "network.target" ];
              wantedBy = [ "multi-user.target" ];

              environment = {
                CONFIG_PATH = cfg.configFile;
              };

              serviceConfig = {
                # TODO: DynamicUser = true; when LoadCredential is used
                User = "1000"; # TODO: make into option
                Group = "1000"; # TODO: make into option
                WorkingDirectory = builtins.dirOf cfg.configFile;
                NoNewPrivileges = true;
                ProtectSystem = "strict";
                ProtectHome = true;
                PrivateTmp = true;
                CapabilityBoundingSet = "";
                Type = "simple";
                ExecStart = "${self.packages.${system}.default}/bin/server";
                Restart = "on-failure";
                RestartSec = 5;
              };
            };
          };
        };
    };
}
