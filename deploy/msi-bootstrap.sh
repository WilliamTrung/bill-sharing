#!/usr/bin/env bash
# Run on MSI as user with docker + repo access (one-time / idempotent).
set -euo pipefail

NGINX_CONF_SRC="${1:-/home/tuf/repo/bill-sharing/deploy/msi-nginx-bill-sharing.conf}"
NGINX_CONF_DST="/home/msi/infra/nginx/conf.d/bill-sharing.conf"
CICD_CONFIG="/home/tuf/cicd-polling-job/config.yaml"
CF_ENV="/home/tuf/infra/cloudflare/.env"
DOMAIN="bill-sharing.williamthanhtrung.id.vn"
DEPLOY_ENV="/home/tuf/dev/shared/bill-sharing/secrets/deploy.env"

if [[ -f "$NGINX_CONF_SRC" ]]; then
  cp "$NGINX_CONF_SRC" "$NGINX_CONF_DST"
  chmod 644 "$NGINX_CONF_DST"
  echo "nginx: installed $NGINX_CONF_DST"
fi

if grep -q "name: bill-sharing" "$CICD_CONFIG" 2>/dev/null; then
  echo "cicd: bill-sharing already in config.yaml"
else
  cp "$CICD_CONFIG" "$CICD_CONFIG.bak.$(date +%Y%m%d%H%M%S)"
  cat >>"$CICD_CONFIG" <<'YAML'

  - name: bill-sharing
    repo_path: bill-sharing
    repo_url: git@github.com:WilliamTrung/bill-sharing.git
    branch: main
    compose_file: docker-compose.yml
    deploy_env: dev/shared/bill-sharing/secrets/deploy.env
    method: compose-build
    migrate: null
YAML
  echo "cicd: appended bill-sharing project"
fi

if [[ -f "$CF_ENV" ]] && grep -q "$DOMAIN" "$CF_ENV"; then
  echo "cloudflare: $DOMAIN already in DOMAINS"
elif [[ -f "$CF_ENV" ]]; then
  cp "$CF_ENV" "$CF_ENV.bak.$(date +%Y%m%d%H%M%S)"
  if grep -q '^DOMAINS=' "$CF_ENV"; then
    sed -i "s|^DOMAINS=\(.*\)|DOMAINS=\1,${DOMAIN}|" "$CF_ENV"
  else
    echo "DOMAINS=${DOMAIN}" >>"$CF_ENV"
  fi
  (cd /home/tuf/infra/cloudflare && docker compose up -d)
  echo "cloudflare: added $DOMAIN and restarted cloudflare-ddns"
fi

if [[ -f "$DEPLOY_ENV" ]] && ! grep -q '^DEPLOY_ENV_FILE=' "$DEPLOY_ENV"; then
  echo "DEPLOY_ENV_FILE=/home/tuf/dev/shared/bill-sharing/secrets/deploy.env" >>"$DEPLOY_ENV"
  echo "deploy.env: set DEPLOY_ENV_FILE"
fi

if docker exec global-nginx nginx -t; then
  docker exec global-nginx nginx -s reload
  echo "nginx: reloaded global-nginx"
fi

echo "bootstrap OK"