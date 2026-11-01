# Deployment (Hostinger VPS)

One VPS runs the four environments with Docker Compose. Caddy terminates HTTPS, routes each API
subdomain to its container and serves the F-CTRD frontend as static files; MongoDB is only
reachable from inside the VPS.

| Branch       | API service    | API URL                        | Frontend (F-CTRD) URL          |
| ------------ | -------------- | ------------------------------ | ------------------------------ |
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
   `scripts/generate_env_files.ps1` builds all of them from the local `.env` (see
   [Generate the env files](#generate-the-env-files)).
   ```bash
   sudo mkdir -p /opt/ctrd/env && sudo chmod 750 /opt/ctrd/env
   # after creating the files:
   sudo chgrp -R github-runner /opt/ctrd/env
   ```
   Set `JM_CTRD_CRON_ENABLED=false` in `dev.env` and `qa.env` so they don't spend the external API quotas.
   Set `JM_CTRD_CORS_ORIGINS` to the frontend of the same environment (e.g. `https://qa.app.<domain>` in `qa.env`).
   Every `JM_CTRD_MDB_HOST` must end in `&replicaSet=rs0` (see [MongoDB replica set](#mongodb-replica-set)).
   The generator also creates the replica set key file `/opt/ctrd/mongo/keyfile`. To create it by hand:
   ```bash
   sudo mkdir -p /opt/ctrd/mongo
   openssl rand -base64 756 | sudo tee /opt/ctrd/mongo/keyfile > /dev/null
   sudo chown 999:999 /opt/ctrd/mongo/keyfile && sudo chmod 400 /opt/ctrd/mongo/keyfile
   ```
   It must exist before the first `up`. If it is missing, Docker creates an empty folder in its place
   and mongod does not start.
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

## Generate the env files

On Windows, from the repo root:

```powershell
.\scripts\generate_env_files.ps1 -Domain <domain> -AcmeEmail <email> | Set-Clipboard
```

Paste the output in the VPS SSH terminal: it creates the six files in `/opt/ctrd/env/`, the replica set
key file `/opt/ctrd/mongo/keyfile` (only if it does not exist yet) and, at the end, prints the root password and the `db.createUser(...)` lines for the Mongo users. Each run generates
new Mongo passwords, so paste the output of a single run and keep the `db.createUser` lines for the
next step (outside the repo, e.g. a password manager). The output contains secrets: never commit it.

The script copies `JM_CTRD_STOCK_API_URL`, `JM_CTRD_API_KEY` and every `JM_CTRD_SECRET_*` to all
environments, turns the cronjobs off in dev and qa, and sets `JM_CTRD_CORS_ORIGINS` to the frontend
of each environment. Sandbox and prod share the API keys and both run the cronjobs.

## MongoDB replica set

The services use transactions (`session.WithTransaction`), and those only work on a replica set.
On a standalone instance they fail with
`Transaction numbers are only allowed on a replica set member or mongos`. The `mongo` service in
`docker-compose.yml` therefore runs as a one-node replica set:

- `--replSet rs0`, plus `--keyFile /etc/mongo/keyfile` because a replica set with authentication
  requires one (the file is mounted from `/opt/ctrd/mongo/keyfile`).
- The healthcheck runs `rs.initiate` the first time with the member `mongo:27017` (the name the API
  containers resolve in the `internal` network). It does nothing on later runs and only reports
  healthy once the node is PRIMARY. The API containers wait for that (`condition: service_healthy`).
- The API URIs (`JM_CTRD_MDB_HOST`) include `replicaSet=rs0`.

Check it from the `deploy/` folder:

```bash
sudo docker compose ps mongo     # STATUS must say (healthy)
sudo docker compose exec mongo mongosh -u root -p '<root password>' --authenticationDatabase admin \
  --quiet --eval "rs.status().members.map(m => m.name + ' ' + m.stateStr)"
# [ 'mongo:27017 PRIMARY' ]
```

Connections from outside the Docker network (SSH tunnel, Compass) must use `directConnection=true`.
Without it the client discovers the member `mongo:27017`, which only resolves inside the VPS.

If Mongo was already started standalone, the `mongo-data` volume can be kept: create the key file,
then deploy again or run `sudo docker compose up -d mongo`. The healthcheck initiates the replica set
on the existing data.

## First deploy

The runners deploy by themselves: nothing is copied or cloned by hand. On each push to an environment
branch the runner checks out the repo in `/home/github-runner/actions-runner/_work/c-trd/c-trd` and
runs Docker Compose from its `deploy/` folder. Never clone the repo again somewhere else and run
`docker compose` there: the project name is fixed (`ctrd`), so both copies would manage the same
containers and Caddy would load the Caddyfile of whichever started it last.

1. **Runners online**: GitHub → repo → Settings → Actions → Runners must show the runner as _Idle_
   (in both repos). If it shows _Offline_, the service is not running; see
   [Troubleshooting](#troubleshooting).
2. **Trigger the deploy**: any push or merged PR into `develop` deploys dev. Jobs that waited for a
   runner expire after 24 h; re-run them from the Actions tab (_Re-run jobs_) or with
   `gh run rerun <run-id> -R joaodemata/c-trd`. No new PR is needed.
3. **Mongo users**: the first `up` starts Mongo empty with the root user from `mongo.env` and the
   healthcheck initiates the replica set `rs0` (the API container waits until Mongo is _healthy_).
   If the deploy fails with `dependency failed to start: container ctrd-mongo-1 is unhealthy`, check
   `sudo docker compose logs mongo` (usually a missing key file or one with wrong permissions).
   Until the environment users exist, `ctrd-<env>` fails to connect and keeps restarting; that is expected.
   Create them with the lines printed by the generator:
   ```bash
   cd /home/github-runner/actions-runner/_work/c-trd/c-trd/deploy
   sudo docker compose exec mongo mongosh -u root -p '<root password>' --authenticationDatabase admin
   ```
   ```js
   use admin
   db.createUser({ user: "ctrd_dev", pwd: "<dev password>", roles: [{ role: "readWrite", db: "ctrd_dev" }] })
   // ... same for ctrd_qa, ctrd_sandbox and ctrd_prod
   ```
   A few seconds later the container reconnects by itself (`restart: unless-stopped`).
4. **Catalogs and super user** of each environment: see the next section.
5. **Check**: `sudo docker compose ps`, `sudo docker compose logs -f ctrd-dev` and open
   `https://dev.api.<domain>` and `https://dev.app.<domain>`.

## Catalogs and super user (once per environment)

New databases are empty. The app needs the `catalogs` collection (visibility, condition types,
statuses, privileges...), and `scripts/create_super_user.js` fails with
`No existe el catalogo USER_PRIVILEGES` if it is missing. The catalogs are copied from the local
MongoDB through an SSH tunnel, then the super user is created through the same tunnel.

**Tools on Windows** (once; reopen PowerShell afterwards):

```powershell
winget install MongoDB.Shell
winget install MongoDB.DatabaseTools
```

**Export the reference data from the local Mongo** (once). `<localdb>` is `JM_CTRD_MDB_NAME` of the
local `.env`. `catalogs` is required; `apis`, `providers` and `assets` are the configuration data,
include them to start each environment with the same setup. Never include `triggers`,
`opportunities`, `conditions`, `users` or `sessions`.

```powershell
mongodump --uri "mongodb://localhost:27017" `
  --nsInclude "<localdb>.catalogs" --nsInclude "<localdb>.apis" `
  --nsInclude "<localdb>.providers" --nsInclude "<localdb>.assets" `
  --archive=ref.archive
```

**Open the tunnel** in a separate window and leave it open. The local port is 27018 because 27017
is used by the local MongoDB; the VPS side is the Mongo published on `127.0.0.1:27017`:

```powershell
ssh -N -L 27018:127.0.0.1:27017 <user>@<vps-ip>
```

**For each environment** (example for dev; repeat with `ctrd_qa`, `ctrd_sandbox` and `ctrd_prod` and
their passwords from `db.createUser`). Use a different super user password in prod:

```powershell
# a) Cargar catalogos y datos de configuracion (renombra <localdb>.* a ctrd_dev.*)
mongorestore --uri "mongodb://ctrd_dev:<dev password>@localhost:27018/?authSource=admin&directConnection=true" `
  --archive=ref.archive --nsFrom "<localdb>.*" --nsTo "ctrd_dev.*"

# b) Super usuario (la contrasena no queda en el historial)
$env:CTRD_SUPER_USER_USERNAME = "<username>"
$secure = Read-Host "Password super user" -AsSecureString
$env:CTRD_SUPER_USER_PASSWORD = [Runtime.InteropServices.Marshal]::PtrToStringBSTR([Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure))
$env:JM_CTRD_MDB_USER = "ctrd_dev"

mongosh "mongodb://ctrd_dev:<dev password>@localhost:27018/ctrd_dev?authSource=admin&directConnection=true" --file scripts/create_super_user.js

Remove-Item Env:CTRD_SUPER_USER_PASSWORD
```

It prints `Indices de users y sessions listos.` and `Super usuario <username> creado con id ...`.
`Authentication failed` means the environment user does not exist yet or its password does not
match the one in `/opt/ctrd/env/<env>.env`.

## Troubleshooting

**Job stuck in "Waiting for a runner to pick up this job"**: the runner is registered but offline.
Start its service (as your sudo user, not as `github-runner`):

```bash
cd /home/github-runner/actions-runner        # F-CTRD: /home/github-runner/actions-runner-fctrd
sudo ./svc.sh install github-runner          # "already installed" is fine
sudo ./svc.sh start
sudo ./svc.sh status                         # active (running) + "Listening for Jobs"
```

If it does not start, read its log:

```bash
sudo journalctl -u 'actions.runner.joaodemata-c-trd.*' -n 50 --no-pager
sudo journalctl -u 'actions.runner.joaodemata-f-ctrd.*' -n 50 --no-pager
```

Usual causes: the folder is not owned by the runner user
(`sudo chown -R github-runner:github-runner /home/github-runner/actions-runner*`) or missing
dependencies (`sudo ./bin/installdependencies.sh` inside the runner folder).

**Caddy lost the frontend sites after a deploy**: the Caddyfile comes from the branch that deployed
last. Promote `develop → qa → sandbox → production` so every branch has the same `deploy/caddy/`.

## Useful commands (from the runner checkout, `deploy/` folder)

```bash
docker compose ps
docker compose logs -f ctrd-prod
docker compose restart ctrd-qa
```

## Backups

Schedule a daily `mongodump` per database and copy it off the VPS (Hostinger snapshots alone are
not a backup of the data).
