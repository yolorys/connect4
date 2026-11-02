#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
for tool in docker kind kubectl; do
  command -v "$tool" >/dev/null || { echo "Missing tool: $tool. See README.md." >&2; exit 1; }
done
docker info >/dev/null
# Snap Docker cannot see kind's normal /tmp. Keep archives beside the project,
# where both tools can access them, and remove them even if image loading fails.
kind_temp_dir="$(mktemp -d "$PWD/kind-tmp.XXXXXX")"
trap 'rm -rf "$kind_temp_dir"' EXIT
if ! kind get clusters | grep -qx connect4; then
  TMPDIR="$kind_temp_dir" kind create cluster --name connect4 --config deploy/kind.yaml --wait 120s
fi
docker build -t connect4:local .
TMPDIR="$kind_temp_dir" kind load docker-image connect4:local --name connect4
rm -rf "$kind_temp_dir"
trap - EXIT
kubectl --context kind-connect4 apply -k deploy/k8s
# Restart after loading the image, including when reusing the :local tag.
kubectl --context kind-connect4 -n connect4 rollout restart deployment/connect4
kubectl --context kind-connect4 -n connect4 rollout status statefulset/postgres --timeout=180s
kubectl --context kind-connect4 -n connect4 rollout status deployment/connect4 --timeout=180s
echo 'Connect Four is ready at http://localhost:8080 (Ctrl+C stops forwarding).'
exec kubectl --context kind-connect4 -n connect4 port-forward --address 127.0.0.1 service/connect4 8080:80
