#!/usr/bin/env bash
# Ejecuta los agentes de c-trd sobre la cola de tesis de un entorno. Lo llama el cron del usuario ctrd-agent los
# jueves en la noche (ver deploy/README.md, seccion Agents). El deploy copia este script y el binario a /opt/ctrd/agents.
#
# Uso: run_agents.sh <dev|qa|sandbox|prod>
set -euo pipefail

ENV_NAME="${1:?uso: run_agents.sh <dev|qa|sandbox|prod>}"
BASE_DIR="/opt/ctrd/agents"
BIN="${BASE_DIR}/bin/agent_runner-${ENV_NAME}"
ENV_FILE="${BASE_DIR}/env/${ENV_NAME}.env"
LOG_DIR="${BASE_DIR}/logs"

mkdir -p "$LOG_DIR"
LOG_FILE="${LOG_DIR}/${ENV_NAME}-$(date -u +%Y%m%d).log"
exec >>"$LOG_FILE" 2>&1
echo "=== $(date -u +%FT%TZ) inicio ${ENV_NAME}"

if [ ! -x "$BIN" ]; then
  echo "No existe ${BIN}: haga un deploy de la rama del entorno"
  exit 1
fi
if [ ! -r "$ENV_FILE" ]; then
  echo "No existe ${ENV_FILE} (copie deploy/env/agents.env.example)"
  exit 1
fi

# Variables del runner (CTRD_AGENT_*) y, si se usa, el token de Claude Code (CLAUDE_CODE_OAUTH_TOKEN)
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

# claude queda en ~/.local/bin con el instalador nativo
export PATH="${HOME}/.local/bin:${PATH}"

# Una sola corrida por entorno a la vez
status=0
flock -n "/tmp/ctrd-agents-${ENV_NAME}.lock" "$BIN" || status=$?
echo "=== $(date -u +%FT%TZ) fin ${ENV_NAME} (exit ${status})"

# Logs de mas de 8 semanas
find "$LOG_DIR" -name '*.log' -mtime +56 -delete
exit "$status"
