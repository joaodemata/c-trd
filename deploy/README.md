# Deployment (Hostinger VPS)

One VPS runs the four environments with Docker Compose. Caddy terminates HTTPS and routes each
subdomain to its container; MongoDB is only reachable from inside the VPS.

| Branch       | Service        | URL                          |
|--------------|----------------|------------------------------|
| `develop`    | `ctrd-dev`     | `https://dev.api.<domain>`   |
| `qa`         | `ctrd-qa`      | `https://qa.api.<domain>`    |
| `sandbox`    | `ctrd-sandbox` | `https://sandbox.api.<domain>` |
| `production` | `ctrd-prod`    | `https://api.<domain>`       |

Changes are promoted only through PRs, in this order (`sandbox` is pre-production):

```
feat-* / fix-*  →  develop  →  qa  →  sandbox  →  production
```

Hotfixes branch off `production` and are merged back into `develop` afterwards.

A push to one of those branches runs `.github/workflows/deploy.yml` on the self-hosted runner in
the VPS: it builds the image (which runs `go vet` and `go test`) and restarts only that environment.

## One-time VPS setup

1. **Docker**: `curl -fsSL https://get.docker.com | sh`
2. **Firewall**: `sudo ufw allow OpenSSH && sudo ufw allow 80,443/tcp && sudo ufw enable`
   (also allow 80/443 in hPanel → VPS → Security → Firewall if a rule set is active).
3. **Env files** in `/opt/ctrd/env/` (never in the repo), based on `deploy/env/*.example`:
   `caddy.env`, `mongo.env`, `dev.env`, `qa.env`, `sandbox.env`, `prod.env`.
   ```bash
   sudo mkdir -p /opt/ctrd/env && sudo chmod 750 /opt/ctrd/env
   ```
   Set `JM_CTRD_CRON_ENABLED=false` in `dev.env` and `qa.env` so they don't spend the external API quotas.
4. **Runner** (GitHub → repo → Settings → Actions → Runners → New self-hosted runner → Linux x64):
   ```bash
   sudo adduser --disabled-password github-runner
   sudo usermod -aG docker github-runner
   sudo chgrp -R github-runner /opt/ctrd/env
   sudo su - github-runner
   mkdir actions-runner && cd actions-runner
   # curl + tar commands shown by GitHub
   ./config.sh --url https://github.com/joaodemata/c-trd --token <TOKEN> --labels ctrd-vps --unattended
   exit
   cd /home/github-runner/actions-runner && sudo ./svc.sh install github-runner && sudo ./svc.sh start
   ```
5. **Repo setting**: Settings → Actions → General → Fork pull request workflows →
   "Require approval for all external contributors". The repo is public: the deploy workflow must
   only trigger on `push`, never on `pull_request`.

## MongoDB users (once per environment)

The first `up` creates the root user from `mongo.env`. Then create one user per environment:

```bash
docker compose -f deploy/docker-compose.yml exec mongo mongosh -u root -p --authenticationDatabase admin
```
```js
use admin
db.createUser({ user: "ctrd_prod", pwd: passwordPrompt(), roles: [{ role: "readWrite", db: "ctrd_prod" }] })
```

Create the super user of each environment with `scripts/create_super_user.js` over an SSH tunnel
(`ssh -L 27017:127.0.0.1:27017 user@vps`).

## Useful commands (from the runner checkout, `deploy/` folder)

```bash
docker compose ps
docker compose logs -f ctrd-prod
docker compose restart ctrd-qa
```

## Backups

Schedule a daily `mongodump` per database and copy it off the VPS (Hostinger snapshots alone are
not a backup of the data).
