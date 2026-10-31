# Deployment (Hostinger VPS)

One VPS runs the four environments with Docker Compose. Caddy terminates HTTPS, routes each API
subdomain to its container and serves the F-CTRD frontend as static files; MongoDB is only
reachable from inside the VPS.

| Branch       | API service    | API URL                        | Frontend (F-CTRD) URL          |
|--------------|----------------|--------------------------------|--------------------------------|
| `develop`    | `ctrd-dev`     | `https://dev.api.<domain>`     | `https://dev.app.<domain>`     |
| `qa`         | `ctrd-qa`      | `https://qa.api.<domain>`      | `https://qa.app.<domain>`      |
| `sandbox`    | `ctrd-sandbox` | `https://sandbox.api.<domain>` | `https://sandbox.app.<domain>` |
| `production` | `ctrd-prod`    | `https://api.<domain>`         | `https://app.<domain>`         |

F-CTRD has the same branches. Its own workflow builds the frontend with the API URL of the branch
and copies `dist/` to `/opt/ctrd/web/<env>`, which Caddy serves read-only.

Changes are promoted only through PRs, in this order (`sandbox` is pre-production):

```
feat-* / fix-*  →  develop  →  qa  →  sandbox  →  production
```

Hotfixes branch off `production` and are merged back into `develop` afterwards.

A push to one of those branches runs `.github/workflows/deploy.yml` on the self-hosted runner in
the VPS: it builds the image (which runs `go vet` and `go test`) and restarts only that environment.

## One-time VPS setup

1. **DNS** (hPanel → Domains → DNS records): one A record to the VPS IP for each of `api`,
   `sandbox.api`, `qa.api`, `dev.api`, `app`, `sandbox.app`, `qa.app` and `dev.app`.
2. **Docker**: `curl -fsSL https://get.docker.com | sh`
3. **Firewall**: `sudo ufw allow OpenSSH && sudo ufw allow 80,443/tcp && sudo ufw enable`
   (also allow 80/443 in hPanel → VPS → Security → Firewall if a rule set is active).
4. **Runner user** with Docker access:
   ```bash
   sudo adduser --disabled-password github-runner
   sudo usermod -aG docker github-runner
   ```
5. **Env files** in `/opt/ctrd/env/` (never in the repo), based on `deploy/env/*.example`:
   `caddy.env`, `mongo.env`, `dev.env`, `qa.env`, `sandbox.env`, `prod.env`.
   ```bash
   sudo mkdir -p /opt/ctrd/env && sudo chmod 750 /opt/ctrd/env
   # after creating the files:
   sudo chgrp -R github-runner /opt/ctrd/env
   ```
   Set `JM_CTRD_CRON_ENABLED=false` in `dev.env` and `qa.env` so they don't spend the external API quotas.
   Set `JM_CTRD_CORS_ORIGINS` to the frontend of the same environment (e.g. `https://qa.app.<domain>` in `qa.env`).
6. **Frontend folder**, written by the F-CTRD runner and served by Caddy:
   ```bash
   sudo mkdir -p /opt/ctrd/web && sudo chown github-runner:github-runner /opt/ctrd/web
   ```
7. **Runners**: on a personal account a self-hosted runner belongs to one repo, so install two
   copies with the same user: one for C-TRD (label `ctrd-vps`) and one for F-CTRD (label `fctrd-vps`).
   C-TRD runner (GitHub → repo → Settings → Actions → Runners → New self-hosted runner → Linux x64):
   ```bash
   sudo su - github-runner
   mkdir actions-runner && cd actions-runner
   # curl + tar commands shown by GitHub
   ./config.sh --url https://github.com/joaodemata/c-trd --token <TOKEN> --labels ctrd-vps --unattended
   exit
   cd /home/github-runner/actions-runner && sudo ./svc.sh install github-runner && sudo ./svc.sh start
   ```
   F-CTRD runner: same steps in `/home/github-runner/actions-runner-fctrd`, with the token from the
   `f-ctrd` repo and `--url https://github.com/joaodemata/f-ctrd --labels fctrd-vps`.
8. **Repo setting** (in both repos): Settings → Actions → General → Fork pull request workflows →
   "Require approval for all external contributors". The repos are public: the deploy workflows
   must only trigger on `push`, never on `pull_request`.

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
