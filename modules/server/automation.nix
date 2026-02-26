{ pkgs, ... }: {
  # ── Fetch GCP Secrets on Boot ──────────────────────────────────────────────
  # GitHub PAT for git push (used by flake-lock-update timer)
  systemd.services."fetch-gcp-secrets" = {
    description = "Fetch GitHub token from GCP Secret Manager";
    wantedBy = [ "multi-user.target" ];
    path = with pkgs; [ curl python3 bash ];
    serviceConfig = {
      Type = "oneshot";
      RemainAfterExit = true;
      ExecStart = pkgs.writeShellScript "fetch-gcp-secrets.sh" ''
        set -euo pipefail
        mkdir -p /var/lib

        # Skip if token already exists
        if [ -f /var/lib/github-token ] && [ -s /var/lib/github-token ]; then
          echo "GitHub token already exists, skipping fetch."
          exit 0
        fi

        PROJECT_ID=$(curl -s -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/project/project-id)
        ACCESS_TOKEN=$(curl -s -H "Metadata-Flavor: Google" "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')

        echo "Fetching github-token from Secret Manager..."
        curl -sf -H "Authorization: Bearer $ACCESS_TOKEN" \
          "https://secretmanager.googleapis.com/v1/projects/$PROJECT_ID/secrets/github-token/versions/latest:access" \
          | python3 -c 'import sys,json,base64; print(base64.b64decode(json.load(sys.stdin)["payload"]["data"]).decode())' > /var/lib/github-token
        chmod 600 /var/lib/github-token
      '';
    };
  };

  # ── Periodic flake.lock update timer ────────────────────────────────────────
  # Every day at 03:00 UTC, update VS Code Insiders feed hashes and push to main
  systemd.services."flake-lock-update" = {
    description = "Update VS Code Insiders feed hashes in flake.lock";
    after = [ "network-online.target" "fetch-gcp-secrets.service" "dotfiles-sync.service" ];
    wants = [ "network-online.target" "fetch-gcp-secrets.service" "dotfiles-sync.service" ];
    path = with pkgs; [ git nix curl jq bash ];
    environment.HOME = "/root";
    serviceConfig = {
      Type = "oneshot";
      WorkingDirectory = "/var/lib/dotfiles";
      ExecStart = pkgs.writeShellScript "update-flake-lock.sh" ''
        set -euo pipefail
        REPO="/var/lib/dotfiles"
        cd "$REPO"

        echo "[flake-update] Starting at $(date -u)"

        # Ensure we are on main and it's up to date
        git checkout -B main origin/main

        # Update only the VS Code Insiders feed inputs
        nix flake lock \
          --update-input vsci-feed-darwin-arm64 \
          --update-input vsci-feed-darwin-x64 \
          --update-input vsci-feed-linux-arm64 \
          --update-input vsci-feed-linux-x64

        # If nothing changed, exit cleanly
        if git diff --quiet flake.lock; then
          echo "[flake-update] flake.lock unchanged — nothing to do."
          exit 0
        fi

        # Commit and push directly to main
        git config user.email "bot@dotfiles-server"
        git config user.name  "dotfiles-bot"
        git add flake.lock
        git commit -m "chore: update VS Code Insiders feed hashes ($(date -u +%Y-%m-%d))"

        # Push using the GITHUB_TOKEN stored on the server
        REMOTE_URL="https://x-access-token:$(cat /var/lib/github-token)@github.com/yuu-th/dotfiles.git"
        git push "$REMOTE_URL" main

        echo "[flake-update] Pushed to main"
      '';
    };
  };

  systemd.timers."flake-lock-update" = {
    wantedBy = [ "timers.target" ];
    timerConfig = { OnCalendar = "03:00:00 UTC"; Persistent = true; };
  };

  # ── Dotfiles Sync on Boot ──────────────────────────────────────────────────
  systemd.services."dotfiles-sync" = {
    description = "Sync dotfiles repo on boot";
    wantedBy = [ "multi-user.target" ];
    path = with pkgs; [ git bash ];
    serviceConfig = {
      Type = "oneshot";
      ExecStart = "${pkgs.bash}/bin/bash -c 'if [ ! -d /var/lib/dotfiles/.git ]; then ${pkgs.git}/bin/git clone https://github.com/yuu-th/dotfiles /var/lib/dotfiles; fi; ${pkgs.git}/bin/git -C /var/lib/dotfiles pull --rebase'";
      RemainAfterExit = true;
    };
  };
}
