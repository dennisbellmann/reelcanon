# Coolify deployment

Reel Canon runs as one application container: the Go API serves the built React
frontend from the same origin. PostgreSQL runs in its own persistent container.

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

The first-time PostgreSQL init script creates a read-only role for the public
API. PostgreSQL has no published host port. Keep the admin app private until a
separate authenticated access path is configured.

To enable the authenticated Cockpit without making it public, build the same
image and add `coolify-admin.override.yaml` as a Compose override in the
Coolify service. Set `ADMIN_USER` and a unique `ADMIN_PASSWORD` in that
service's environment. The override binds port 8081 to host loopback only;
access it through an SSH tunnel, for example `ssh -L 8081:127.0.0.1:8081 root@your-server`, then open `http://127.0.0.1:8081/canons`. The admin connects
to the persistent production database, so visibility changes affect the live
catalog.
