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

1. Open http://localhost:8080/api/superset/operation/ after logging in through the Super App.
2. Sign in with the local Superset admin user from `.env`.
3. Go to Settings, then Database Connections.
4. Add a PostgreSQL database using:

   ```text
   postgresql+psycopg2://bi_user:bi_password@mock-data-warehouse:5432/bi_warehouse
   ```

5. Test the connection, save it, then create datasets from the seeded tables.

## Architecture

Visual references:

- Mermaid architecture diagram: `docs/architecture.mmd`
- Mermaid authentication sequence: `docs/auth-sequence.mmd`
- Static demo storyboard: `docs/demo-storyboard.svg`

### English

This platform separates the Super App integration layer from the Superset source
repository. `bi-engine-platform` owns the UI shells, Go proxy/backend, local
session management, Docker Compose orchestration, Keycloak setup, MongoDB, Nginx,
and the local Public/Operation zone simulation. The Superset fork owns Superset
source code, patches, image builds, map/custom driver changes, and Superset-only
configuration.

The Operation Zone does not use Superset guest tokens or embedded-Superset
authentication. A user authenticates with Keycloak through Authorization Code
Flow. Keycloak is expected to authenticate the user against LDAP user federation
and the configured organizational unit (OU). After the callback, the Go backend
exchanges the authorization code for Keycloak tokens, creates its own Super App
session, and stores encrypted Keycloak access/refresh tokens in MongoDB.

The browser receives only an `HttpOnly` same-origin session cookie. The raw
session id is not stored in MongoDB; MongoDB stores a hash of that session id.
Access and refresh tokens are encrypted with AES-GCM before being persisted. The
encryption key material is derived from `BACKEND_SESSION_SECRET`, so that secret
must be strong and environment-specific.

When the user opens report design from the Super App menu, the
`super-app-superset-ui` micro frontend is loaded. Its iframe points to the Go
proxy path:

```text
/api/superset/operation/
```

The iframe does not call Superset directly. Browser requests include the Super
App session cookie, the Go backend loads the session from MongoDB, decrypts or
refreshes the Keycloak access token, and proxies the request to
`superset-operation:8088` with:

```text
Authorization: Bearer <Keycloak access token>
```

Superset Operation uses a custom security manager:

```text
infra/superset/operation/keycloak_proxy_security_manager.py
```

That security manager validates the forwarded token against Keycloak's OpenID
Connect token introspection endpoint. If the token is valid, Superset creates or
loads the matching local user and serves the page without showing the Superset
login screen.

The intended Operation Zone request flow is:

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

### فارسی

این پلتفرم لایه یکپارچه‌سازی Super App را از سورس Superset جدا نگه می‌دارد.
مخزن `bi-engine-platform` مالک UIها، بک‌اند و proxy با Go، مدیریت session محلی،
Docker Compose، تنظیمات محلی Keycloak، MongoDB، Nginx و شبیه‌سازی محیط‌های
Public و Operation است. مخزن fork شده Superset فقط مالک سورس Superset، patchها،
ساخت image، تغییرات نقشه، driverها و تنظیمات اختصاصی Superset است.

در محیط Operation از guest token و مکانیزم embedded Superset استفاده نمی‌شود.
کاربر از طریق Authorization Code Flow وارد Keycloak می‌شود. Keycloak باید از
طریق LDAP user federation و OU تعریف‌شده، کاربر را احراز هویت کند. بعد از
callback، بک‌اند Go کد authorization را با توکن‌های Keycloak تعویض می‌کند،
برای کاربر یک session مستقل در Super App می‌سازد و access token و refresh token
را به صورت رمزنگاری‌شده در MongoDB ذخیره می‌کند.

مرورگر فقط یک cookie از نوع `HttpOnly` و هم‌دامنه دریافت می‌کند. مقدار خام
session id در MongoDB ذخیره نمی‌شود؛ فقط hash آن ذخیره می‌شود. توکن‌های
Keycloak قبل از ذخیره شدن با AES-GCM رمزنگاری می‌شوند. کلید رمزنگاری از
`BACKEND_SESSION_SECRET` مشتق می‌شود، بنابراین این مقدار باید قوی و مخصوص همان
محیط باشد.

وقتی کاربر از منوی Super App گزینه طراحی گزارش را انتخاب می‌کند، micro frontend
به نام `super-app-superset-ui` باز می‌شود. iframe داخل این micro frontend به
مسیر proxy بک‌اند Go اشاره می‌کند:

```text
/api/superset/operation/
```

iframe مستقیما به Superset وصل نمی‌شود. درخواست‌های مرورگر cookie مربوط به
session در Super App را همراه خود دارند. بک‌اند Go با استفاده از این cookie،
session را از MongoDB پیدا می‌کند، access token کاربر را decrypt یا در صورت نیاز
refresh می‌کند، و درخواست را با header زیر به سرویس `superset-operation:8088`
ارسال می‌کند:

```text
Authorization: Bearer <Keycloak access token>
```

در سمت Superset Operation یک security manager اختصاصی استفاده می‌شود:

```text
infra/superset/operation/keycloak_proxy_security_manager.py
```

این security manager توکن ارسال‌شده توسط Go proxy را از طریق endpoint
introspection در Keycloak اعتبارسنجی می‌کند. اگر توکن معتبر باشد، Superset کاربر
متناظر را ایجاد یا بارگذاری می‌کند و صفحه را بدون نمایش login داخلی Superset به
کاربر نشان می‌دهد.

جریان هدف در محیط Operation به شکل زیر است:

```text
کاربر
  -> ورود به Super App با Authorization Code Flow در Keycloak
  -> احراز هویت Keycloak بر اساس LDAP / OU
  -> تعویض authorization code با توکن‌های Keycloak در بک‌اند Go
  -> ذخیره توکن‌های رمزنگاری‌شده در MongoDB
  -> دریافت cookie از نوع HttpOnly برای session در Super App
  -> باز شدن iframe در super-app-superset-ui روی مسیر /api/superset/operation/
  -> بارگذاری session توسط Go، decrypt/refresh کردن access token
  -> proxy شدن درخواست iframe به superset-operation با Authorization: Bearer <token>
  -> اعتبارسنجی توکن توسط Superset Operation از طریق Keycloak introspection
  -> ایجاد/بارگذاری کاربر متناظر در Superset و نمایش صفحه بدون login مجدد
```

### Operation Zone Auth Details

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
