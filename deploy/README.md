# Coolify deployment

Reel Canon runs as one application container: the Go API serves the built React
frontend from the same origin. PostgreSQL runs in its own persistent container.

The following steps describe the initial service setup and a manual recovery
path. After the CI/CD setup below is enabled, normal releases come from `main`
through GitHub Actions.

1. Copy this repository to `/data/coolify/source/reelcanon` on the VPS. Do not
   copy `.env`, `.local`, or `.git`.
2. In Coolify, create a **Docker Compose Empty** service and paste
   `deploy/coolify-compose.yaml`.
3. Set strong random `POSTGRES_PASSWORD` and `API_DATABASE_PASSWORD` values in
   that service's environment variables. Use hexadecimal values so the database
   URL needs no escaping.
4. Add a domain to the `app` service using internal port `8080`, then deploy.
5. Run the `migrate` and `import-canon` services once with the `maintenance`
   profile from the service directory. The sample snapshot is a historical
   archive; TMDb enrichment can be configured separately.

To share one hostname between applications, set the Coolify build variable
`VITE_BASE_PATH` to a path with leading and trailing slashes (for example
`/reelcanon/`) and rebuild Reel Canon. In the app service's **Domains** field,
route `https://your-host/reelcanon` to port `8080`. Coolify strips the matched
prefix by default, which lets the Go server continue serving its normal `/`
and `/api` routes. A route for `/` can then point to another app; the more
specific `/reelcanon` route takes priority. Keep the trailing slash in
`VITE_BASE_PATH` even if Coolify displays the route without it.

The first-time PostgreSQL init script creates a read-only role for the public
API. PostgreSQL has no published host port. Keep the admin app private until a
separate authenticated access path is configured.

## CI/CD from `main`

The GitHub Actions workflow tests pull requests and pushes. When the deploy gate
is enabled, a successful push or manual run on `main` packages only the files
tracked by that exact commit, uploads the archive over SSH, and asks the local
Coolify API to rebuild this service. Coolify itself stays private; no public
Coolify domain or GitHub webhook is required. `make deploy` queues the same
workflow for the latest commit already on `main` (requires the GitHub CLI `gh`
to be installed and authenticated).

One-time setup:

1. Create a dedicated VPS user for deployment and install one SSH key for it.
   The user needs write access to `/data/coolify/source/reelcanon` so the
   workflow can replace that build context. Do not use the root SSH account.
2. In Coolify, enable API access and create two team API tokens: one with only
   `deploy` permission and one with only `read` permission. Store them as the
   GitHub repository secrets `COOLIFY_DEPLOY_TOKEN` and `COOLIFY_READ_TOKEN`.
   Restrict Coolify's API IP allowlist to the Docker gateway addresses used by
   the local caller when possible. Disable API access again when this workflow
   is not in use.
3. Add these repository secrets:
   - `VPS_DEPLOY_KEY`: the private half of the dedicated SSH key.
   - `COOLIFY_DEPLOY_TOKEN`: the deploy-only Coolify API token.
   - `COOLIFY_READ_TOKEN`: the read-only Coolify API token used to poll status.
4. Add these repository variables:
   - `VPS_DEPLOY_HOST`: the VPS hostname or IP.
   - `VPS_DEPLOY_USER`: the dedicated deployment user.
   - `VPS_SSH_KNOWN_HOSTS`: the verified SSH host-key line for the VPS. Verify
     the host-key fingerprint with the provider before saving it.
   - `COOLIFY_SERVICE_UUID`: the UUID shown in the Coolify service URL.
   - `REELCANON_DEPLOY_ENABLED`: set to `true` only after the secrets and
     variables above are configured.

The deployment account must be able to call `http://127.0.0.1:8000` on the
VPS, where Coolify listens locally, and the existing Coolify context directory
must be writable by that account. The release script preserves `.env`, `.local`
and `.git` if any are present, but these should not be part of the build
context. The PostgreSQL volume is not replaced. The workflow waits for Coolify
to report the deployment result and fails if the deployment fails or times out.

For a release, merge a tested pull request into `main`; the deploy job then runs
after the main CI job succeeds. To redeploy the current `main` commit manually,
run `make deploy`.

To enable the authenticated Cockpit without making it public, build the same
image and add `coolify-admin.override.yaml` as a Compose override in the
Coolify service. Set `ADMIN_USER` and a unique `ADMIN_PASSWORD` in that
service's environment. The override binds port 8081 to host loopback only;
access it through an SSH tunnel, for example `ssh -L 8081:127.0.0.1:8081 root@your-server`, then open `http://127.0.0.1:8081/canons`. The admin connects
to the persistent production database, so visibility changes affect the live
catalog.
