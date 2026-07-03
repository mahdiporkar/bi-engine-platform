# BI Engine Platform

Local integration platform for the Super App and BI Engine POC.

This repository owns the integration layer:

- React Super App UI
- React Superset micro frontend
- Go Fiber backend/proxy
- MongoDB session management
- Keycloak local identity setup
- Nginx local gateway
- Docker Compose orchestration
- Local Public Zone / Operation Zone simulation

Superset source code is intentionally not vendored here. Superset customizations and image builds belong in:

https://github.com/mahdiporkar/superset

## Repository Layout

```text
bi-engine-platform/
  README.md
  .env.example
  docker-compose.yml
  apps/
    super-app-ui/
    super-app-superset-ui/
  services/
    super-app-backend/
  infra/
    nginx/
    keycloak/
    mongo/
    mock-data-warehouse/
      init/
    superset/
      public/
        superset_config.py
      operation/
        superset_config.py
```

## Superset Repository Boundary

Do not copy Superset source code into this repository.

Build the local Superset image from the Superset fork:

```powershell
git clone https://github.com/mahdiporkar/superset.git
cd superset
docker build -t bi-engine-superset:local .
```

If Docker Hub blocks the base images required by the Superset fork Dockerfile,
build a local overlay image from an already available Superset base while using
the local fork source as the build context:

```powershell
docker build `
  -f E:\Project\github-lab-poc\infra\superset\Dockerfile.local-source `
  -t bi-engine-superset:local `
  E:\Project\superset
```

This copies and installs the Python Superset backend from `E:\Project\superset`
into the local image. Rebuild the image after changing the local Superset source.

If the Public Zone and Operation Zone need separate images, build them from the same Superset fork and tag them as:

```powershell
docker build -t bi-engine-superset-public:local .
docker build -t bi-engine-superset-operation:local .
```

Then update `SUPERSET_PUBLIC_IMAGE` and `SUPERSET_OPERATION_IMAGE` in `.env`.

## Local Run

1. Copy the environment template:

   ```powershell
   Copy-Item .env.example .env
   ```

2. Build the Superset image from `mahdiporkar/superset` as described above.

3. Start the platform:

   ```powershell
   docker compose up --build
   ```

4. Open:

   - Gateway / Super App: http://localhost:8080
   - Super App UI direct: http://localhost:5173
   - Superset micro frontend direct: http://localhost:5174
   - Backend direct: http://localhost:8090
   - Keycloak: http://localhost:8081
   - Superset Public Zone direct: http://localhost:8088
   - Superset Operation Zone via Go proxy: http://localhost:8080/api/superset/operation/

## Mock Data Warehouse

The local POC includes an internal-only PostgreSQL service named `mock-data-warehouse`.
It is not exposed to the host by default. Superset containers can reach it through the
Docker network.

Connection details:

- Host: `mock-data-warehouse`
- Port: `5432`
- Database: `bi_warehouse`
- Username: `bi_user`
- Password: `bi_password`
- SQLAlchemy URI: `postgresql+psycopg2://bi_user:bi_password@mock-data-warehouse:5432/bi_warehouse`

Seed SQL files live in `infra/mock-data-warehouse/init/` and create:

- `sales_orders`
- `customers`
- `monthly_kpis`

The sample data supports basic Superset charts for sales by province, monthly sales
trends, top product categories, order status distribution, and customer segment
analysis.

To add the connection manually in the Superset Operation Zone:

1. Open http://localhost:8089 or http://localhost:8080/superset/operation/
2. Sign in with the local Superset admin user from `.env`.
3. Go to Settings, then Database Connections.
4. Add a PostgreSQL database using:

   ```text
   postgresql+psycopg2://bi_user:bi_password@mock-data-warehouse:5432/bi_warehouse
   ```

5. Test the connection, save it, then create datasets from the seeded tables.

## Operation Superset Auth Flow

The intended operation-zone flow is:

```text
User
  -> Super App Authorization Code Flow with Keycloak
  -> Keycloak authenticates against LDAP / OU federation
  -> Go backend exchanges the authorization code for Keycloak tokens
  -> Go backend stores encrypted tokens in MongoDB
  -> Browser receives an HttpOnly same-origin Super App session cookie
  -> super-app-superset-ui iframe opens /api/superset/operation/
  -> Go backend loads the session, decrypts/refreshes the Keycloak access token
  -> Go backend proxies iframe traffic to superset-operation with Authorization: Bearer <token>
  -> Superset Operation validates the token against Keycloak introspection
  -> Superset creates/loads the matching local user and serves the page without Superset login
```

No guest-token or embedded-Superset flow is used for the Operation Zone.

Local proxy route:

```text
/api/superset/operation/* -> super-app-backend -> superset-operation:8088
```

Superset Operation uses `infra/superset/operation/keycloak_proxy_security_manager.py`
as `CUSTOM_SECURITY_MANAGER`. It expects a bearer token forwarded by the Go proxy
and validates it through Keycloak's OpenID Connect token introspection endpoint.

Backend auth endpoints:

- `GET /api/auth/login` starts Keycloak Authorization Code Flow.
- `GET /api/auth/callback` handles the code exchange and creates the local session.
- `GET /api/auth/me` returns the current Super App session user.
- `POST /api/auth/logout` deletes the local session.

Session and token storage:

- Browser cookie: `BI_ENGINE_SESSION`, `HttpOnly`, same-origin.
- Mongo collection: `sessions`.
- Mongo stores a hash of the session id, not the raw browser cookie value.
- Access and refresh tokens are encrypted with AES-GCM.
- Encryption key material is derived from `BACKEND_SESSION_SECRET`.
- Use a strong non-default `BACKEND_SESSION_SECRET` before sharing the environment.

Required Keycloak settings:

- Keycloak must be configured with LDAP user federation for the target OU.
- `KEYCLOAK_BASE_URL`, `KEYCLOAK_REALM`, `KEYCLOAK_CLIENT_ID`, and
  `KEYCLOAK_CLIENT_SECRET` must point to a confidential client allowed to
  introspect tokens.
- Users with any role in `SUPERSET_KEYCLOAK_ADMIN_ROLES` become Superset `Admin`;
  other valid users get `SUPERSET_KEYCLOAK_DEFAULT_ROLE`.

## Local Zones

The POC runs two Superset containers from locally built images:

- `superset-public` uses `infra/superset/public/superset_config.py`
- `superset-operation` uses `infra/superset/operation/superset_config.py`

Both configs are integration-time settings only. Superset code patches, map changes, Oracle drivers, OAuth changes, and image build logic remain in the Superset fork.
