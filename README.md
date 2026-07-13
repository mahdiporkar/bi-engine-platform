# BI Engine Platform

Local integration platform for the Super App and BI Engine POC.

This repository owns the integration layer:

- React Super App UI
- React Superset micro frontend
- Go Fiber backend/proxy
- MongoDB session management
- Keycloak local identity setup
- Keycloak PostgreSQL metadata database for stable local SSO state
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

Do not copy Superset source code into this repository. Keep the fork checked out
locally and let Docker Compose build Superset from that source tree:

```powershell
git clone https://github.com/mahdiporkar/superset.git
```

Set the fork path in `.env`:

```powershell
SUPERSET_SOURCE_DIR=E:\Project\superset
```

The `superset-public` service builds the Dockerfile inside `SUPERSET_SOURCE_DIR`.
`superset-operation` uses that same local source-built image. They do not use the
prebuilt `apache/superset` image:

```text
bi-engine-superset:local-source
```

Rebuild these images after changing the local Superset fork:

```powershell
docker compose build superset-public
```

## Local Run

1. Copy the environment template:

   ```powershell
   Copy-Item .env.example .env
   ```

2. Confirm `SUPERSET_SOURCE_DIR` points to the local Superset fork.

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

The `superset-operation` service provisions this connection automatically during
startup with:

```text
infra/superset/operation/provision_operation_dwh.py
```

```text
Database name: Mock Data Warehouse
SQLAlchemy URI: postgresql+psycopg2://bi_user:bi_password@mock-data-warehouse:5432/bi_warehouse
```

The connection is controlled by:

```text
SUPERSET_OPERATION_DWH_DATABASE_NAME
SUPERSET_OPERATION_DWH_SQLALCHEMY_URI
```

If you need to add or verify it manually in the Superset Operation Zone:

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
App session cookie. The Go backend loads the session from MongoDB, decrypts or
refreshes the public Keycloak access token, exchanges that token with the
Operation Keycloak, and proxies the request to `superset-operation:8088` with the
Operation-issued token:

```text
Authorization: Bearer <Operation Keycloak access token>
```

Superset Operation uses a custom security manager:

```text
infra/superset/operation/keycloak_proxy_security_manager.py
```

That security manager validates the forwarded Operation token against the
Operation Keycloak OpenID Connect token introspection endpoint. If the token is
valid, Superset creates or loads the matching local user and serves the page
without showing the Superset login screen.

The intended Operation Zone request flow is:

```text
User
  -> Super App Authorization Code Flow with Keycloak
  -> Keycloak authenticates against LDAP / OU federation
  -> Go backend exchanges the authorization code for Keycloak tokens
  -> Go backend stores encrypted tokens in MongoDB
  -> Browser receives an HttpOnly same-origin Super App session cookie
  -> super-app-superset-ui iframe opens /api/superset/operation/
  -> Go backend loads the session, decrypts/refreshes the public Keycloak access token
  -> Go backend exchanges the public token with Operation Keycloak
  -> Go backend proxies iframe traffic to superset-operation with Authorization: Bearer <operation token>
  -> Superset Operation validates the operation token against Operation Keycloak introspection
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
session را از MongoDB پیدا می‌کند، access token صادرشده توسط Keycloak عمومی را
decrypt یا در صورت نیاز refresh می‌کند، سپس همان توکن عمومی را با Keycloak محیط
Operation از طریق Token Exchange تعویض می‌کند. خروجی این مرحله یک access token
جدید است که issuer و client آن متعلق به محیط Operation است. بک‌اند Go درخواست را
با header زیر به سرویس `superset-operation:8088` ارسال می‌کند:

```text
Authorization: Bearer <Operation Keycloak access token>
```

در سمت Superset Operation یک security manager اختصاصی استفاده می‌شود:

```text
infra/superset/operation/keycloak_proxy_security_manager.py
```

این security manager توکن عملیات ارسال‌شده توسط Go proxy را از طریق endpoint
introspection در Keycloak محیط Operation اعتبارسنجی می‌کند. بنابراین Superset
Operation توکن Keycloak عمومی را مستقیما trust نمی‌کند؛ Superset فقط توکنی را
می‌پذیرد که Keycloak عملیات آن را معتبر اعلام کند. اگر توکن معتبر باشد، Superset
کاربر متناظر را ایجاد یا بارگذاری می‌کند و صفحه را بدون نمایش login داخلی
Superset به کاربر نشان می‌دهد.

جریان هدف در محیط Operation به شکل زیر است:

```text
کاربر
  -> ورود به Super App با Authorization Code Flow در Keycloak
  -> احراز هویت Keycloak بر اساس LDAP / OU
  -> تعویض authorization code با توکن‌های Keycloak در بک‌اند Go
  -> ذخیره توکن‌های رمزنگاری‌شده در MongoDB
  -> دریافت cookie از نوع HttpOnly برای session در Super App
  -> باز شدن iframe در super-app-superset-ui روی مسیر /api/superset/operation/
  -> بارگذاری session توسط Go، decrypt/refresh کردن access token عمومی
  -> Token Exchange بین Go proxy و Keycloak محیط Operation
  -> proxy شدن درخواست iframe به superset-operation با Authorization: Bearer <operation token>
  -> اعتبارسنجی توکن عملیات توسط Superset Operation از طریق introspection در Keycloak عملیات
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
and validates it through the Operation Keycloak OpenID Connect token introspection
endpoint.

Backend auth endpoints:

- `GET /api/auth/login` starts Keycloak Authorization Code Flow.
- `GET /api/auth/callback` handles the code exchange and creates the local session.
- `GET /api/auth/me` returns the current Super App session user.
- `POST /api/auth/logout` deletes the local session.

Keycloak URL split:

- `KEYCLOAK_BASE_URL`, `KEYCLOAK_REALM`, `KEYCLOAK_CLIENT_ID`, and
  `KEYCLOAK_CLIENT_SECRET` describe the public Keycloak used by the Super App
  Authorization Code Flow, userinfo, and refresh-token flow.
- `KEYCLOAK_PUBLIC_URL` is the browser URL used only for the Authorization Code
  Flow redirect. In local compose this should normally be `http://localhost:8081`.
- `OPERATION_KEYCLOAK_BASE_URL`, `OPERATION_KEYCLOAK_REALM`,
  `OPERATION_KEYCLOAK_CLIENT_ID`, and `OPERATION_KEYCLOAK_CLIENT_SECRET`
  describe the Operation Keycloak client used by the Go proxy for Token Exchange
  and by Superset Operation for token introspection.
- `OPERATION_KEYCLOAK_TOKEN_EXCHANGE_ENABLED=true` makes the Go proxy exchange
  the public access token before forwarding requests to Superset Operation.

Local Keycloak persistence:

- The local Keycloak service uses the `keycloak-db` PostgreSQL container.
- This avoids the instability of the embedded H2 dev database during repeated
  Docker restarts.
- The local database settings are controlled by `KEYCLOAK_DB_NAME`,
  `KEYCLOAK_DB_USER`, and `KEYCLOAK_DB_PASSWORD`.

Operation routing:

- `/api/superset/operation/*` is the canonical iframe/proxy route.
- `/superset/operation/*` also routes to the Go backend proxy, so users cannot
  bypass the Super App session path through the local nginx gateway.

Session and token storage:

- Browser cookie: `BI_ENGINE_SESSION`, `HttpOnly`, same-origin.
- Mongo collection: `sessions`.
- Mongo stores a hash of the session id, not the raw browser cookie value.
- Access and refresh tokens are encrypted with AES-GCM.
- Encryption key material is derived from `BACKEND_SESSION_SECRET`.
- Use a strong non-default `BACKEND_SESSION_SECRET` before sharing the environment.

Required Keycloak settings:

- Keycloak must be configured with LDAP user federation for the target OU.
- LDAP/OU mapping belongs in Keycloak, not in Superset. Configure the LDAP
  provider, bind DN, user search base, user object classes, username/email
  mappers, and group/role mapper in the `bi-engine` realm.
- After LDAP login, Keycloak must issue tokens containing a stable `sub`,
  `preferred_username`, and optionally `email`, `given_name`, `family_name`, and
  realm/client roles.
- The public Keycloak client must allow Authorization Code Flow and refresh
  tokens for the Super App.
- Token Exchange must be enabled on Keycloak. In local Docker Compose this is
  done with `start-dev --features=token-exchange --import-realm`.
- The public `super-app` client must include `superset-operation` in the access
  token audience. The local realm import does this with the
  `superset-operation-audience` protocol mapper.
- The Operation Keycloak client must be confidential, must be allowed to receive
  token-exchange requests from the Go proxy, and must be allowed to introspect
  the exchanged Operation tokens.
- Users with any role in `SUPERSET_KEYCLOAK_ADMIN_ROLES` become Superset `Admin`;
  other valid users get `SUPERSET_KEYCLOAK_DEFAULT_ROLE`.

### مستندات فارسی تغییرات Token Exchange

نیازمندی جدید این است که Superset محیط Operation توکن صادرشده توسط Keycloak
عمومی را مستقیما معتبر نداند. مسیر درست این است که Go proxy بعد از احراز هویت
کاربر در محیط عمومی، هنگام ورود به Superset Operation یک Token Exchange با
Keycloak عملیات انجام دهد و فقط توکن صادرشده توسط Keycloak عملیات را به Superset
Operation ارسال کند.

کد این نیازمندی در فایل زیر اضافه شده است:

```text
services/super-app-backend/cmd/server/main.go
```

رفتار جدید backend به این شکل است:

- کاربر همچنان با `GET /api/auth/login` وارد Keycloak عمومی می‌شود.
- callback در `GET /api/auth/callback` authorization code را با توکن‌های Keycloak عمومی تعویض می‌کند.
- access token و refresh token عمومی به صورت رمزنگاری‌شده در MongoDB ذخیره می‌شوند.
- وقتی iframe مسیر `/api/superset/operation/` را باز می‌کند، Go proxy session را از cookie پیدا می‌کند.
- اگر access token عمومی منقضی شده باشد، با refresh token عمومی آن را refresh می‌کند.
- قبل از proxy کردن request به `superset-operation`، تابع `exchangeOperationAccessToken` اجرا می‌شود.
- این تابع با grant type زیر به token endpoint Keycloak عملیات درخواست می‌زند:

```text
urn:ietf:params:oauth:grant-type:token-exchange
```

payload اصلی درخواست Token Exchange شامل این مقادیر است:

```text
subject_token=<public keycloak access token>
subject_token_type=urn:ietf:params:oauth:token-type:access_token
requested_token_type=urn:ietf:params:oauth:token-type:access_token
audience=<operation audience, optional>
requested_issuer=<operation issuer, optional>
client_id=<operation keycloak client id>
client_secret=<operation keycloak client secret>
```

اگر Keycloak عملیات درخواست را قبول کند، یک access token جدید برمی‌گرداند. این
توکن جدید همان توکنی است که در header زیر به Superset Operation ارسال می‌شود:

```text
Authorization: Bearer <Operation Keycloak access token>
```

در نتیجه Superset Operation به جای trust کردن issuer عمومی، توکن را با client
عملیات و endpoint introspection عملیات بررسی می‌کند. فایل
`infra/superset/operation/keycloak_proxy_security_manager.py` همین کار را انجام
می‌دهد: header `Authorization` را می‌خواند، token introspection را روی Keycloak
عملیات صدا می‌زند، و فقط اگر پاسخ introspection دارای `active=true` باشد کاربر را
در Superset ایجاد یا load می‌کند.

متغیرهای محیطی جدید:

- `OPERATION_KEYCLOAK_BASE_URL`: آدرس داخلی Keycloak عملیات برای backend و Superset Operation.
- `OPERATION_KEYCLOAK_REALM`: realm محیط عملیات.
- `OPERATION_KEYCLOAK_CLIENT_ID`: client محرمانه عملیات که Go proxy با آن token exchange می‌زند.
- `OPERATION_KEYCLOAK_CLIENT_SECRET`: secret همان client عملیات.
- `OPERATION_KEYCLOAK_TOKEN_EXCHANGE_ENABLED`: اگر `true` باشد، مسیر Operation بدون token exchange به Superset توکن نمی‌فرستد.
- `OPERATION_KEYCLOAK_TOKEN_EXCHANGE_AUDIENCE`: مقدار audience برای توکن عملیات؛ معمولا client مربوط به Superset Operation است.
- `OPERATION_KEYCLOAK_TOKEN_EXCHANGE_REQUESTED_ISSUER`: در سناریوهایی که Keycloak عملیات issuer مشخص می‌خواهد استفاده می‌شود.
- `OPERATION_KEYCLOAK_TOKEN_EXCHANGE_REQUESTED_TOKEN_TYPE`: نوع توکن خروجی؛ مقدار پیش‌فرض access token است.

در `docker-compose.yml` سرویس `super-app-backend` این envها را دریافت می‌کند.
سرویس `superset-operation` هم برای introspection از همین envهای عملیات استفاده
می‌کند، اما به دلیل نام‌گذاری داخل security manager، این مقادیر در container
Superset با نام‌های `KEYCLOAK_BASE_URL`، `KEYCLOAK_REALM`، `KEYCLOAK_CLIENT_ID`
و `KEYCLOAK_CLIENT_SECRET` تزریق می‌شوند. بنابراین از دید Superset Operation،
Keycloak اصلی همان Keycloak عملیات است.

در `infra/keycloak/realm-bi-engine.json` برای محیط local یک client نمونه با نام
`superset-operation` اضافه شده است. در محیط واقعی باید در Keycloak عملیات این
تنظیمات قطعی انجام شود:

- client عملیات باید confidential باشد.
- client عملیات باید مجاز به introspection توکن‌های عملیات باشد.
- policy یا permission مربوط به Token Exchange باید اجازه دهد Go proxy توکن عمومی را با توکن عملیات تعویض کند.
- claimهای لازم مانند `sub`، `preferred_username`، `email` و roleها باید در توکن عملیات وجود داشته باشند.
- اگر roleهای مدیریتی مانند `bi-admin` یا `superset-admin` لازم است، mapperهای realm/client role باید در Keycloak عملیات تنظیم شوند.

جمع‌بندی معماری جدید:

```text
Public Keycloak token
  -> Go proxy
  -> Token Exchange with Operation Keycloak
  -> Operation Keycloak token
  -> Superset Operation
  -> Operation Keycloak introspection
```

پس پاسخ دقیق به سؤال معماری این است: بله، برای محیط Operation اکنون Token
Exchange در Go proxy انجام می‌شود و Superset Operation فقط توکن صادرشده یا
تأییدشده توسط Keycloak عملیات را معتبر می‌بیند.

### مستندات فارسی کدها و هدف متدها

این بخش برای مرور فنی کدهای انجام‌شده نوشته شده است. تمرکز اصلی روی فایل
`services/super-app-backend/cmd/server/main.go` است، چون Go backend نقطه اصلی
اعتماد، session، refresh token، Token Exchange و proxy به Superset است.

#### ساختار کلی فایل `main.go`

فایل `main.go` چند مسئولیت اصلی دارد:

- خواندن تنظیمات runtime از environment variableها.
- اتصال به MongoDB برای ذخیره sessionها.
- ساخت codec رمزنگاری برای ذخیره امن access token و refresh token.
- اجرای OAuth Authorization Code Flow با Keycloak عمومی.
- ساخت session داخلی Super App و ارسال cookie امن به مرورگر.
- refresh کردن توکن عمومی در صورت نزدیک شدن به انقضا.
- اجرای Token Exchange با Keycloak عملیات قبل از ورود به Superset Operation.
- proxy کردن requestهای مسیر Public و Operation به سرویس واقعی Superset Operation.
- بازنویسی redirectهای Superset تا کاربر از مسیر proxy خارج نشود.

#### importها در `main.go`

```go
import (
    "context"
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "io"
    "log"
    "net/http"
    "net/url"
    "os"
    "strings"
    "time"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/proxy"
    "go.mongodb.org/mongo-driver/bson"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)
```

هدف هر import:

- `context`: برای timeout گذاشتن روی اتصال MongoDB، queryها و updateها.
- `crypto/aes` و `crypto/cipher`: برای رمزنگاری AES-GCM توکن‌ها قبل از ذخیره در MongoDB.
- `crypto/rand`: برای تولید session id، state و nonce امن.
- `crypto/sha256`: برای hash کردن session id و ساخت کلید 256 بیتی از secret.
- `encoding/base64`: برای تبدیل بایت‌های random، hash و ciphertext به string قابل ذخیره.
- `encoding/json`: برای decode کردن پاسخ token endpoint و userinfo.
- `errors`: برای ساخت errorهای قابل فهم در flowهای auth.
- `io`: برای خواندن امن random bytes و bodyهای رمزنگاری.
- `log`: برای stop کردن سرویس در خطاهای critical هنگام startup.
- `net/http`: برای call زدن به Keycloak token endpoint و userinfo.
- `net/url`: برای ساخت payloadهای form-urlencoded در OAuth و Token Exchange.
- `os`: برای خواندن environment variableها.
- `strings`: برای trim کردن URLها، بررسی prefixها و پردازش headerها.
- `time`: برای انقضای session، انقضای token و timeoutها.
- `fiber`: فریم‌ورک HTTP اصلی backend.
- `proxy`: middleware مخصوص Fiber برای forward کردن requestها به Superset.
- `bson`, `mongo`, `options`: driver رسمی MongoDB برای ذخیره و خواندن sessionها.

#### struct `config`

`config` تمام تنظیمات runtime را در یک ساختار واحد نگه می‌دارد. این struct دو نوع
تنظیم دارد: تنظیماتی که به frontend قابل نمایش هستند و تنظیماتی که secret هستند و
در JSON خروجی مخفی می‌شوند.

فیلدهای اصلی:

- `Port`: پورتی که backend روی آن listen می‌کند.
- `MongoURI`: آدرس MongoDB؛ با `json:"-"` از خروجی `/config` حذف شده است.
- `MongoDatabase`: نام database برای collection مربوط به sessionها.
- `SessionSecret`: secret اصلی رمزنگاری؛ با `json:"-"` مخفی است.
- `CookieSecure`: تعیین می‌کند cookie فقط روی HTTPS ارسال شود یا نه.
- `BackendPublicURL`: آدرس عمومی backend برای ساخت redirect URI.
- `KeycloakBaseURL`: آدرس داخلی Keycloak عمومی برای callهای backend.
- `KeycloakPublicURL`: آدرس قابل دسترس از مرورگر برای redirect به صفحه login.
- `KeycloakRealm`: realm عمومی که Super App با آن login می‌کند.
- `KeycloakClientID`: client عمومی Super App.
- `KeycloakClientSecret`: secret client عمومی؛ از JSON خروجی حذف شده است.
- `OperationKeycloakBaseURL`: آدرس داخلی Keycloak عملیات.
- `OperationKeycloakRealm`: realm عملیات.
- `OperationKeycloakClientID`: client محرمانه عملیات برای Token Exchange.
- `OperationKeycloakClientSecret`: secret client عملیات؛ از JSON خروجی حذف شده است.
- `OperationTokenExchangeEnabled`: روشن یا خاموش بودن Token Exchange برای Operation.
- `OperationTokenExchangeAudience`: مقدار audience مورد انتظار در توکن عملیات.
- `OperationTokenExchangeRequestedIssuer`: issuer هدف در سناریوهای چند issuer.
- `OperationTokenExchangeRequestedTokenType`: نوع توکن خروجی؛ پیش‌فرض access token است.
- `SupersetPublicURL`: آدرس Superset عمومی برای UI، صفحه‌ها و static assetهای مسیر `/superset/public/*`.
- `SupersetOperationURL`: آدرس upstream واقعی Superset عملیات که requestهای API، JSON، chart data، query و اجرای سرویس به آن forward می‌شوند.

نکته امنیتی: فیلدهایی مثل `MongoURI`، `SessionSecret` و client secretها نباید از
endpoint `/config` به frontend بروند. به همین دلیل `json:"-"` دارند.

#### struct `session`

`session` مدل ذخیره‌شده در MongoDB است.

- `ID`: شناسه session در MongoDB است، اما مقدار خام cookie نیست؛ hash شده است.
- `UserID`: مقدار `sub` کاربر در Keycloak.
- `Username`: نام کاربری قابل نمایش یا قابل mapping در Superset.
- `Email`: ایمیل کاربر اگر در claimها وجود داشته باشد.
- `Zone`: zone کاربر؛ در flow فعلی مقدار `operation` ثبت می‌شود.
- `EncryptedAccessToken`: access token عمومی Keycloak به صورت رمزنگاری‌شده.
- `EncryptedRefreshToken`: refresh token عمومی Keycloak به صورت رمزنگاری‌شده.
- `TokenExpiresAt`: زمان انقضای access token عمومی.
- `CreatedAt`: زمان ایجاد session.
- `ExpiresAt`: زمان انقضای session داخلی Super App.

هدف این طراحی این است که مرورگر فقط cookie داشته باشد و توکن‌های واقعی داخل
MongoDB، آن هم به صورت encrypted، نگهداری شوند.

#### struct `tokenResponse`

این struct پاسخ استاندارد Keycloak token endpoint را نگه می‌دارد:

- `AccessToken`: توکن اصلی برای callها.
- `RefreshToken`: توکن refresh برای گرفتن access token جدید.
- `ExpiresIn`: مدت اعتبار access token بر حسب ثانیه.
- `TokenType`: معمولا `Bearer`.

همین struct برای سه حالت استفاده می‌شود:

- exchange کردن authorization code با توکن عمومی.
- refresh کردن access token عمومی.
- exchange کردن توکن عمومی با توکن عملیات.

#### struct `userInfo`

این struct پاسخ userinfo از Keycloak عمومی را مدل می‌کند:

- `Subject`: شناسه پایدار کاربر یا همان `sub`.
- `PreferredUsername`: نام کاربری ترجیحی.
- `Email`: ایمیل کاربر.

این اطلاعات برای ساخت session داخلی Super App استفاده می‌شوند.

#### constantهای cookie

```go
const (
    sessionCookieName  = "BI_ENGINE_SESSION"
    stateCookieName    = "BI_ENGINE_OAUTH_STATE"
    returnToCookieName = "BI_ENGINE_RETURN_TO"
)
```

- `BI_ENGINE_SESSION`: cookie اصلی session داخلی Super App.
- `BI_ENGINE_OAUTH_STATE`: مقدار state برای جلوگیری از CSRF در OAuth callback.
- `BI_ENGINE_RETURN_TO`: مسیر مقصد بعد از login، مثلا برگشت به micro frontend.

#### متد `main`

`main` نقطه شروع backend است و همه routeها در آن تعریف می‌شوند.

منطق خط‌به‌خط:

1. `cfg := loadConfig()` تنظیمات را از environment variableها می‌خواند.
2. `context.WithTimeout(..., 10*time.Second)` برای اتصال اولیه MongoDB timeout می‌گذارد.
3. `mongo.Connect(...)` اتصال MongoDB را ایجاد می‌کند.
4. اگر اتصال fail شود، سرویس با `log.Fatalf` متوقف می‌شود، چون بدون MongoDB session management ممکن نیست.
5. `defer mongoClient.Disconnect(...)` تضمین می‌کند هنگام خروج، اتصال MongoDB بسته شود.
6. `sessions := mongoClient.Database(...).Collection("sessions")` collection مربوط به sessionها را انتخاب می‌کند.
7. `newTokenCodec(cfg.SessionSecret)` codec رمزنگاری AES-GCM را می‌سازد.
8. اگر secret نامعتبر باشد، سرویس start نمی‌شود تا توکن‌ها بدون رمزنگاری امن ذخیره نشوند.
9. `fiber.New(...)` اپلیکیشن HTTP را ایجاد می‌کند.
10. routeهای health، config، auth، logout، proxy و session تعریف می‌شوند.
11. در انتها `app.Listen(":" + cfg.Port)` backend را روی پورت تنظیم‌شده اجرا می‌کند.

#### route `GET /health`

هدف: بررسی سلامت backend و MongoDB.

منطق:

- یک context دو ثانیه‌ای ساخته می‌شود.
- `mongoClient.Ping` اجرا می‌شود.
- اگر MongoDB پاسخ ندهد، status برابر `503 Service Unavailable` برمی‌گردد.
- اگر MongoDB سالم باشد، خروجی `{"status":"ok"}` برمی‌گردد.

این endpoint برای Docker healthcheck، مانیتورینگ و تست سریع مناسب است.

#### route `GET /config`

هدف: ارسال تنظیمات غیرحساس به frontend.

منطق:

- کل `cfg` به JSON تبدیل می‌شود.
- فیلدهایی که `json:"-"` دارند، مثل secretها و MongoURI، حذف می‌شوند.

این route نباید secret برگرداند و طراحی فعلی همین را رعایت می‌کند.

#### route `GET /superset/zones`

هدف: اعلام zoneهای قابل استفاده برای UI.

خروجی:

- zone عمومی با مسیر نمایشی `/superset/public/`.
- zone عملیات با مسیر proxy داخلی `/api/superset/operation/`.

نکته: برای Operation، frontend نباید مستقیم به Superset وصل شود؛ باید از مسیر Go
proxy استفاده کند.

#### route `GET /auth/login`

هدف: شروع Authorization Code Flow با Keycloak عمومی.

منطق خط‌به‌خط:

1. `randomString(32)` یک state امن می‌سازد.
2. state در cookie با نام `BI_ENGINE_OAUTH_STATE` ذخیره می‌شود.
3. cookie مربوط به state از نوع `HttpOnly` است تا JavaScript به آن دسترسی نداشته باشد.
4. `Secure` از config خوانده می‌شود و در محیط production باید `true` باشد.
5. `SameSite=Lax` باعث می‌شود callback OAuth همچنان کار کند و ریسک CSRF کمتر شود.
6. اگر query parameter به نام `return_to` وجود داشته باشد، با `safeReturnTo` اعتبارسنجی می‌شود.
7. اگر مسیر مقصد امن باشد، در cookie `BI_ENGINE_RETURN_TO` ذخیره می‌شود.
8. `url.Values` پارامترهای authorization request را می‌سازد.
9. `client_id` برابر client عمومی Super App است.
10. `response_type=code` یعنی از Authorization Code Flow استفاده می‌شود.
11. `scope=openid profile email` برای گرفتن claimهای پایه OIDC است.
12. `redirect_uri` از `cfg.redirectURI()` ساخته می‌شود.
13. `state` برای اعتبارسنجی callback ارسال می‌شود.
14. کاربر با `302 Found` به endpoint login در Keycloak عمومی redirect می‌شود.

#### route `GET /auth/callback`

هدف: دریافت authorization code از Keycloak عمومی و ساخت session داخلی.

منطق خط‌به‌خط:

1. مقدار `state` query با cookie `BI_ENGINE_OAUTH_STATE` مقایسه می‌شود.
2. اگر state خالی یا متفاوت باشد، callback رد می‌شود.
3. مقدار `code` از query خوانده می‌شود.
4. اگر code وجود نداشته باشد، request نامعتبر است.
5. `exchangeCode(cfg, code)` code را با توکن‌های Keycloak عمومی تعویض می‌کند.
6. `fetchUserInfo(cfg, tokens.AccessToken)` اطلاعات کاربر را از userinfo می‌گیرد.
7. access token عمومی با `codec.encrypt` رمزنگاری می‌شود.
8. refresh token عمومی هم با `codec.encrypt` رمزنگاری می‌شود.
9. `randomString(32)` یک session id خام برای cookie تولید می‌کند.
10. `now := time.Now().UTC()` زمان پایه را برای created/expires می‌سازد.
11. اگر `preferred_username` خالی باشد، `sub` به عنوان username استفاده می‌شود.
12. یک document از نوع `session` ساخته می‌شود.
13. مقدار `ID` برابر `sessionIDHash(sessionID)` است، نه مقدار خام cookie.
14. `TokenExpiresAt` بر اساس `ExpiresIn` توکن عمومی محاسبه می‌شود.
15. `ExpiresAt` برای session داخلی ۸ ساعت بعد تنظیم می‌شود.
16. session در MongoDB ذخیره می‌شود.
17. cookie اصلی `BI_ENGINE_SESSION` با مقدار خام session id به مرورگر داده می‌شود.
18. cookie از نوع `HttpOnly` است.
19. cookie مربوط به state پاک می‌شود.
20. cookie مربوط به return_to خوانده و سپس پاک می‌شود.
21. اگر return_to خالی باشد، کاربر به `/superset-mfe/` می‌رود.
22. در نهایت کاربر redirect می‌شود.

این route توکن عملیات تولید نمی‌کند. توکن عملیات فقط هنگام ورود به مسیر Superset
Operation و داخل `proxySuperset` ساخته می‌شود.

#### route `GET /auth/me`

هدف: اعلام وضعیت login کاربر به frontend.

منطق:

- `loadSession` با cookie فعلی session را از MongoDB می‌خواند.
- اگر session معتبر نباشد، `authenticated=false` با status 401 برمی‌گردد.
- اگر session معتبر باشد، شناسه کاربر، username، email و زمان انقضا برمی‌گردد.

#### route `POST /auth/logout`

هدف: حذف session داخلی Super App.

منطق:

- اگر cookie session وجود داشته باشد، hash آن محاسبه می‌شود.
- document متناظر از MongoDB حذف می‌شود.
- cookie مرورگر با `MaxAge=-1` پاک می‌شود.
- خروجی `{"ok":true}` برمی‌گردد.

نکته: این logout فعلا session داخلی را حذف می‌کند. اگر logout از Keycloak هم
لازم باشد، باید redirect یا back-channel logout جداگانه اضافه شود.

#### route `ALL /superset/operation/*`

هدف: proxy کردن همه متدهای HTTP مربوط به Superset Operation.

منطق:

- هر request زیر این مسیر وارد `proxySuperset` می‌شود.
- `upstreamBaseURL` برابر `cfg.SupersetOperationURL` است.
- `publicPrefix` برابر `/api/superset/operation` است.
- `zone` برابر `operation` است.

چون zone عملیات است، داخل `proxySuperset` Token Exchange انجام می‌شود.

#### route `ALL /superset/public/*`

هدف: جدا کردن UI عمومی از سرویس‌دهی عملیاتی.

در معماری فعلی Superset Public فقط نقش UI دارد. بنابراین Go proxy برای مسیر
بیرونی `/superset/public/*` بر اساس نوع path تصمیم می‌گیرد:

```text
/superset/public/                  -> Go proxy -> superset-public:8088
/superset/public/static/*           -> Go proxy -> superset-public:8088
/superset/public/superset/welcome/*  -> Go proxy -> superset-public:8088
/superset/public/api/*              -> Go proxy -> superset-operation:8088
/superset/public/superset/*_json*   -> Go proxy -> superset-operation:8088
/superset/public/superset/results*  -> Go proxy -> superset-operation:8088
```

رفتار این route برای درخواست‌های UI:

- upstream برابر `cfg.SupersetPublicURL` است.
- prefix واقعی upstream برابر `/superset/public` است.
- `zone` برابر `public` است.
- Token Exchange عملیات انجام نمی‌شود.

رفتار این route برای درخواست‌های API/JSON/data:

- `zone` برابر `operation` ارسال می‌شود.
- upstream برابر `cfg.SupersetOperationURL` است.
- prefix واقعی upstream برابر `/api/superset/operation` است.
- Token Exchange با Keycloak عملیات انجام می‌شود.
- توکن عمومی مستقیما به Superset داده نمی‌شود.
- header نهایی برای Superset Operation شامل `Authorization: Bearer <operation token>` است.

#### route `POST /sessions`

هدف: ساخت session تستی یا legacy برای توسعه local.

منطق:

- body شامل `userId` و `zone` خوانده می‌شود.
- اگر `userId` خالی باشد، `local-admin` استفاده می‌شود.
- اگر `zone` خالی باشد، `public` استفاده می‌شود.
- یک session ساده در MongoDB ذخیره می‌شود.

نکته: این route برای flow اصلی SSO/Token Exchange استفاده نمی‌شود و بیشتر کاربرد
توسعه‌ای دارد.

#### متد `proxySuperset`

هدف: تبدیل request مرورگر به request معتبر برای Superset.

پارامترها:

- `c`: context درخواست Fiber.
- `cfg`: تنظیمات runtime.
- `sessions`: collection مربوط به sessionها در MongoDB.
- `codec`: ابزار decrypt/encrypt توکن‌ها.
- `upstreamBaseURL`: آدرس container یا upstream Superset.
- `upstreamPrefix`: prefix واقعی داخل Superset upstream.
- `publicPrefix`: prefix بیرونی که کاربر در مرورگر می‌بیند.
- `zone`: مشخص می‌کند request باید با سیاست امنیتی Operation پردازش شود یا نه.

منطق خط‌به‌خط:

1. `bearerToken(c)` بررسی می‌کند آیا request خودش Bearer token دارد یا نه.
2. اگر token در header یا cookie نبود، `sessionAccessToken` اجرا می‌شود.
3. `sessionAccessToken` با cookie داخلی، session را از MongoDB می‌خواند.
4. اگر session معتبر نباشد و zone عملیات باشد، request با 401 رد می‌شود.
5. اگر token پیدا شد و zone عملیات باشد و Token Exchange روشن باشد، `exchangeOperationAccessToken` اجرا می‌شود.
6. اگر Token Exchange fail شود، request با خطای 401 رد می‌شود.
7. اگر Token Exchange موفق باشد، token عمومی با token عملیات جایگزین می‌شود.
8. اگر بعد از همه این مراحل token وجود نداشته باشد و bypass غیرفعال باشد، request رد می‌شود.
9. اگر token وجود داشته باشد، header زیر روی request تنظیم می‌شود:

```text
Authorization: Bearer <token>
```

10. `wildcard := c.Params("*")` بخش dynamic مسیر را می‌خواند.
11. اگر مسیر با slash تمام شده باشد، slash حفظ می‌شود تا Superset redirect اشتباه نسازد.
12. `target` با ترکیب `upstreamBaseURL`، `upstreamPrefix` و `wildcard` ساخته می‌شود؛ برای UI مسیر واقعی upstream همان `/superset/public/*` است، اما برای API/JSON/data مسیر بیرونی `/superset/public/*` به `/api/superset/operation/*` نگاشت می‌شود.
13. query string اصلی request به target اضافه می‌شود.
14. header `X-BI-Engine-Proxy` برای trace/debug تنظیم می‌شود.
15. `proxy.Do(c, target)` request را به Superset forward می‌کند.
16. `rewriteSupersetRedirect` بعد از پاسخ، headerهای redirect را از prefix واقعی upstream به prefix بیرونی مرورگر تبدیل می‌کند.
17. اگر همه چیز موفق باشد، پاسخ Superset به مرورگر برمی‌گردد.

نکته کلیدی: در Operation، Superset هیچ وقت توکن عمومی را دریافت نمی‌کند؛ اگر
Token Exchange روشن باشد، همیشه توکن عملیات در header قرار می‌گیرد.

#### متد `isSupersetDataRequest`

هدف: تشخیص اینکه یک request زیر مسیر public فقط UI است یا باید به Superset
Operation برود.

این متد فقط روی route زیر اثر دارد:

```text
/superset/public/*
```

منطق:

1. مقدار wildcard مسیر خوانده می‌شود.
2. مسیر trim و lowercase می‌شود.
3. اگر مسیر با prefixهای عملیاتی شروع شود، خروجی `true` است.
4. اگر مسیر عملیاتی نباشد، خروجی `false` است و request به Superset Public می‌رود.

prefixهای عملیاتی شامل این موارد هستند:

```text
/api/
/superset/explore_json
/superset/results
/superset/slice_json
/superset/log
/superset/csv
/superset/excel
/superset/sqllab
/superset/queries
/superset/sql_json
/savedqueryviewapi/
/sqllab/
```

پس اگر مرورگر صفحه UI را بخواهد، request از `superset-public` پاسخ می‌گیرد؛ اما
اگر همان UI درخواست chart data، API، JSON، SQL Lab یا export بفرستد، Go proxy آن
را به `superset-operation` می‌فرستد.

#### struct و متدهای `tokenCodec`

`tokenCodec` مسئول رمزنگاری و decrypt توکن‌هاست.

##### `newTokenCodec`

هدف: ساخت AES-GCM codec از `SESSION_SECRET`.

منطق:

1. اگر secret خالی باشد یا با `change-me` شروع شود، خطا می‌دهد.
2. `sha256.Sum256` از secret یک کلید ۳۲ بایتی می‌سازد.
3. `aes.NewCipher` block cipher را ایجاد می‌کند.
4. `cipher.NewGCM` حالت GCM را می‌سازد.
5. خروجی، codec قابل استفاده برای encrypt/decrypt است.

چرا AES-GCM؟ چون هم محرمانگی می‌دهد و هم integrity؛ یعنی اگر ciphertext دستکاری
شود، decrypt fail می‌شود.

##### `encrypt`

هدف: رمزنگاری access token و refresh token قبل از ذخیره در MongoDB.

منطق:

1. یک nonce تصادفی با اندازه مورد نیاز GCM ساخته می‌شود.
2. `io.ReadFull(rand.Reader, nonce)` nonce را از random امن پر می‌کند.
3. `codec.aead.Seal` plaintext را encrypt و authenticate می‌کند.
4. nonce ابتدای ciphertext قرار می‌گیرد.
5. خروجی با Base64 URL-safe encode می‌شود.

##### `decrypt`

هدف: باز کردن توکن رمزنگاری‌شده از MongoDB.

منطق:

1. مقدار ذخیره‌شده از Base64 decode می‌شود.
2. اگر طول ciphertext از اندازه nonce کمتر باشد، داده نامعتبر است.
3. nonce از ابتدای payload جدا می‌شود.
4. ciphertext واقعی بعد از nonce جدا می‌شود.
5. `codec.aead.Open` decrypt و verify را انجام می‌دهد.
6. اگر داده دستکاری شده باشد یا secret اشتباه باشد، decrypt fail می‌شود.

#### متد `randomString`

هدف: تولید string امن برای state و session id.

منطق:

- به اندازه `size` بایت random امن تولید می‌شود.
- خروجی با Base64 URL-safe تبدیل به string می‌شود.

این متد برای security-sensitive valueها استفاده می‌شود، نه random معمولی.

#### متد `sessionIDHash`

هدف: جلوگیری از ذخیره raw session id در MongoDB.

منطق:

- مقدار session id خام با SHA-256 hash می‌شود.
- hash با Base64 URL-safe ذخیره می‌شود.

اگر MongoDB leak شود، مهاجم مقدار cookie واقعی را مستقیم در اختیار ندارد.

#### متد `loadSession`

هدف: خواندن session معتبر از روی cookie مرورگر.

منطق:

1. cookie با نام `BI_ENGINE_SESSION` خوانده می‌شود.
2. اگر cookie وجود نداشته باشد، خطا برمی‌گردد.
3. context دو ثانیه‌ای برای query ساخته می‌شود.
4. مقدار cookie hash می‌شود.
5. MongoDB دنبال document با `_id` برابر hash و `expiresAt` بزرگ‌تر از زمان فعلی می‌گردد.
6. اگر document پیدا شود، session برگردانده می‌شود.
7. اگر پیدا نشود یا expire شده باشد، خطا برمی‌گردد.

#### متد `sessionAccessToken`

هدف: گرفتن access token عمومی معتبر از session داخلی.

منطق خط‌به‌خط:

1. `loadSession` session را از MongoDB می‌خواند.
2. اگر session وجود نداشته باشد، خطا برمی‌گردد.
3. اگر access token بیشتر از یک دقیقه اعتبار داشته باشد، همان توکن decrypt و برگردانده می‌شود.
4. اگر token نزدیک انقضا باشد، refresh token از MongoDB decrypt می‌شود.
5. `refreshAccessToken` با Keycloak عمومی تماس می‌گیرد.
6. access token جدید encrypt می‌شود.
7. اگر Keycloak refresh token جدید هم برگرداند، آن هم encrypt می‌شود.
8. MongoDB با توکن‌های جدید و `tokenExpiresAt` جدید update می‌شود.
9. access token عمومی جدید برگردانده می‌شود.

این متد هنوز توکن عمومی را برمی‌گرداند. تبدیل آن به توکن عملیات در
`proxySuperset` و با `exchangeOperationAccessToken` انجام می‌شود.

#### متد `exchangeCode`

هدف: تعویض authorization code با توکن‌های Keycloak عمومی.

منطق:

- `grant_type=authorization_code` تنظیم می‌شود.
- `code` دریافتی از callback ارسال می‌شود.
- `redirect_uri` باید دقیقا با redirect URI مرحله login یکی باشد.
- request از طریق `tokenRequest` به Keycloak عمومی ارسال می‌شود.

#### متد `refreshAccessToken`

هدف: گرفتن access token عمومی جدید با refresh token عمومی.

منطق:

- `grant_type=refresh_token` تنظیم می‌شود.
- refresh token رمزگشایی‌شده ارسال می‌شود.
- `tokenRequest` درخواست را به token endpoint عمومی می‌فرستد.

#### متد `tokenRequest`

هدف: اجرای call مشترک به token endpoint عمومی Keycloak.

منطق خط‌به‌خط:

1. `client_id` عمومی Super App به payload اضافه می‌شود.
2. `client_secret` عمومی Super App به payload اضافه می‌شود.
3. request از نوع `POST` به مسیر `/realms/<realm>/protocol/openid-connect/token` ساخته می‌شود.
4. body به صورت `application/x-www-form-urlencoded` ارسال می‌شود.
5. request با `http.DefaultClient.Do` اجرا می‌شود.
6. response body با `defer res.Body.Close()` بسته می‌شود.
7. اگر status code خارج از بازه 200 تا 299 باشد، خطا برمی‌گردد.
8. JSON پاسخ در `tokenResponse` decode می‌شود.
9. اگر `access_token` خالی باشد، پاسخ نامعتبر است.
10. در حالت موفق، token response برگردانده می‌شود.

این متد برای Keycloak عمومی است، نه عملیات.

#### متد `exchangeOperationAccessToken`

هدف: تبدیل access token عمومی به access token عملیات.

این متد پیاده‌سازی اصلی نیازمندی جدید است.

منطق خط‌به‌خط:

1. یک `url.Values` جدید ساخته می‌شود.
2. `grant_type` برابر مقدار استاندارد Token Exchange قرار می‌گیرد:

```text
urn:ietf:params:oauth:grant-type:token-exchange
```

3. `subject_token` برابر access token عمومی کاربر است.
4. `subject_token_type` برابر access token تنظیم می‌شود.
5. اگر `OperationTokenExchangeRequestedTokenType` تنظیم شده باشد، به payload اضافه می‌شود.
6. اگر `OperationTokenExchangeAudience` تنظیم شده باشد، audience توکن عملیات مشخص می‌شود.
7. اگر `OperationTokenExchangeRequestedIssuer` تنظیم شده باشد، issuer هدف مشخص می‌شود.
8. در نهایت `operationTokenRequest` اجرا می‌شود.

خروجی این متد توکنی است که باید توسط Superset Operation معتبر دیده شود.

#### متد `operationTokenRequest`

هدف: ارسال payload Token Exchange به token endpoint Keycloak عملیات.

منطق خط‌به‌خط:

1. `client_id` مربوط به client عملیات به payload اضافه می‌شود.
2. `client_secret` مربوط به client عملیات به payload اضافه می‌شود.
3. request به آدرس `operationKeycloakRealmURL()/protocol/openid-connect/token` ساخته می‌شود.
4. header `Content-Type` روی `application/x-www-form-urlencoded` قرار می‌گیرد.
5. request اجرا می‌شود.
6. response body بسته می‌شود.
7. اگر status code موفق نباشد، خطای Token Exchange عملیات برمی‌گردد.
8. پاسخ JSON در `tokenResponse` decode می‌شود.
9. اگر access token خالی باشد، پاسخ نامعتبر است.
10. در حالت موفق، توکن عملیات برگردانده می‌شود.

تفاوت مهم با `tokenRequest`: این متد از client و realm عملیات استفاده می‌کند.

#### متد `fetchUserInfo`

هدف: گرفتن اطلاعات کاربر از Keycloak عمومی بعد از login.

منطق:

1. request از نوع `GET` به `/userinfo` ساخته می‌شود.
2. access token عمومی در header `Authorization` قرار می‌گیرد.
3. request اجرا می‌شود.
4. اگر status code موفق نباشد، خطا برمی‌گردد.
5. پاسخ در struct `userInfo` decode می‌شود.
6. اگر `sub` خالی باشد، پاسخ نامعتبر است.
7. اطلاعات کاربر برگردانده می‌شود.

این متد به Superset ربط مستقیم ندارد؛ برای ساخت session داخلی استفاده می‌شود.

#### متدهای URL در `config`

##### `keycloakRealmURL`

آدرس داخلی realm عمومی را می‌سازد:

```text
<KEYCLOAK_BASE_URL>/realms/<KEYCLOAK_REALM>
```

برای token request، refresh و userinfo عمومی استفاده می‌شود.

##### `operationKeycloakRealmURL`

آدرس داخلی realm عملیات را می‌سازد:

```text
<OPERATION_KEYCLOAK_BASE_URL>/realms/<OPERATION_KEYCLOAK_REALM>
```

برای Token Exchange با Keycloak عملیات استفاده می‌شود.

##### `keycloakPublicRealmURL`

آدرس عمومی realm عمومی را می‌سازد:

```text
<KEYCLOAK_PUBLIC_URL>/realms/<KEYCLOAK_REALM>
```

برای redirect مرورگر به صفحه login Keycloak استفاده می‌شود.

##### `redirectURI`

redirect URI backend را می‌سازد:

```text
<BACKEND_PUBLIC_URL>/auth/callback
```

این مقدار باید با redirect URI تعریف‌شده در client عمومی Keycloak سازگار باشد.

#### متد `bearerToken`

هدف: پیدا کردن access token از request.

منطق:

1. header `Authorization` خوانده می‌شود.
2. اگر با `Bearer ` شروع شود، مقدار token از آن جدا و trim می‌شود.
3. اگر header نبود، cookieهای `KC_ACCESS_TOKEN`، `KEYCLOAK_ACCESS_TOKEN` و `access_token` بررسی می‌شوند.
4. اگر هیچ token پیدا نشود، string خالی برمی‌گردد.

در flow اصلی iframe، معمولا token از session داخلی خوانده می‌شود، نه از header
مرورگر.

#### متد `safeReturnTo`

هدف: جلوگیری از open redirect.

منطق:

- مقدار ورودی trim می‌شود.
- اگر خالی باشد، رد می‌شود.
- اگر با `/` شروع نشود، رد می‌شود.
- اگر با `//` شروع شود، رد می‌شود.
- اگر شامل `://` باشد، رد می‌شود.
- فقط مسیرهای داخلی مثل `/superset-mfe/` پذیرفته می‌شوند.

#### متد `rewriteSupersetRedirect`

هدف: اصلاح redirectهای Superset برای باقی ماندن کاربر پشت Go proxy.

مشکل چیست؟ Superset ممکن است در header `Location` آدرس داخلی خودش یا مسیر خام
برگرداند. اگر همان مقدار به مرورگر برسد، کاربر ممکن است از مسیر proxy خارج شود.

منطق:

1. header `Location` از پاسخ Superset خوانده می‌شود.
2. اگر `Location` خالی باشد، متد کاری نمی‌کند.
3. آدرس upstream و prefix عمومی trim می‌شوند.
4. اگر redirect با upstream داخلی شروع شود، upstream حذف و prefix عمومی جایگزین می‌شود.
5. اگر redirect نسبی باشد، prefix عمومی به ابتدای آن اضافه می‌شود.
6. اگر redirect absolute path باشد اما prefix عمومی نداشته باشد، prefix اضافه می‌شود.

این متد باعث می‌شود مسیرهایی مثل `/login/` یا آدرس داخلی container به مسیر
درست proxy تبدیل شوند.

#### متد `loadConfig`

هدف: ساخت config کامل از environment variableها.

منطق:

- برای هر env یک fallback local تعریف شده است.
- تنظیمات عمومی Keycloak از `KEYCLOAK_*` خوانده می‌شوند.
- تنظیمات Keycloak عملیات از `OPERATION_KEYCLOAK_*` خوانده می‌شوند.
- اگر envهای عملیات تنظیم نشده باشند، برای سازگاری local از envهای عمومی fallback می‌گیرند.
- `OPERATION_KEYCLOAK_TOKEN_EXCHANGE_ENABLED` به bool تبدیل می‌شود.
- مقدار پیش‌فرض `OPERATION_KEYCLOAK_TOKEN_EXCHANGE_REQUESTED_TOKEN_TYPE` برابر access token است.

نکته: در production بهتر است envهای عملیات همیشه جدا و صریح تنظیم شوند تا اشتباها
Superset Operation به Keycloak عمومی وصل نشود.

#### متد `env`

هدف: خواندن environment variable با fallback.

منطق:

- `os.Getenv(key)` مقدار env را می‌خواند.
- اگر مقدار خالی باشد، fallback برگردانده می‌شود.
- اگر مقدار وجود داشته باشد، همان مقدار استفاده می‌شود.

#### فایل `infra/superset/operation/keycloak_proxy_security_manager.py`

این فایل سمت Superset Operation اجرا می‌شود و تعیین می‌کند Superset چطور request
های دارای bearer token را به user داخلی تبدیل کند.

##### کلاس `KeycloakProxySecurityManager`

این کلاس از `SupersetSecurityManager` ارث‌بری می‌کند و request-level auth را
پیاده‌سازی می‌کند.

##### متد `request_loader`

هدف: لاگین کردن کاربر در Superset بر اساس bearer token ارسالی از Go proxy.

منطق:

1. `_bearer_token(request)` توکن را از header `Authorization` استخراج می‌کند.
2. اگر token وجود نداشته باشد، `None` برمی‌گردد و Superset کاربر را لاگین‌شده نمی‌بیند.
3. `_introspect(token)` توکن را از Keycloak عملیات اعتبارسنجی می‌کند.
4. اگر introspection خالی باشد یا `active=true` نداشته باشد، request رد می‌شود.
5. username از `preferred_username`، سپس `username`، سپس `sub` انتخاب می‌شود.
6. اگر username پیدا نشود، user ساخته نمی‌شود.
7. email از claim `email` خوانده می‌شود یا مقدار fallback ساخته می‌شود.
8. first name از `given_name` یا username گرفته می‌شود.
9. last name از `family_name` یا string خالی گرفته می‌شود.
10. role با `_role_name(claims)` تعیین می‌شود.
11. اگر user قبلا در Superset وجود داشته باشد، همان user برگردانده می‌شود.
12. اگر user وجود نداشته باشد، با `add_user` ساخته می‌شود.

##### متد `_bearer_token`

هدف: استخراج bearer token از header.

منطق:

- header `Authorization` خوانده می‌شود.
- اگر با `bearer ` شروع شود، بخش token جدا می‌شود.
- اگر header معتبر نباشد، string خالی برمی‌گردد.

##### متد `_introspection_url`

هدف: ساخت آدرس introspection در Keycloak عملیات.

منطق:

- اگر `KEYCLOAK_INTROSPECTION_URL` تنظیم شده باشد، همان استفاده می‌شود.
- در غیر این صورت `KEYCLOAK_BASE_URL` و `KEYCLOAK_REALM` خوانده می‌شوند.
- آدرس نهایی به این شکل ساخته می‌شود:

```text
<KEYCLOAK_BASE_URL>/realms/<KEYCLOAK_REALM>/protocol/openid-connect/token/introspect
```

در container `superset-operation` این envها از `OPERATION_KEYCLOAK_*` تزریق شده‌اند،
پس منظور از `KEYCLOAK_BASE_URL` در آن container همان Keycloak عملیات است.

##### متد `_introspect`

هدف: اعتبارسنجی token با Keycloak عملیات.

منطق:

1. client id و secret از env خوانده می‌شوند.
2. payload شامل `token`، `client_id` و `client_secret` ساخته می‌شود.
3. request از نوع `POST` به introspection endpoint ساخته می‌شود.
4. content type برابر `application/x-www-form-urlencoded` است.
5. request با timeout پنج ثانیه اجرا می‌شود.
6. response JSON decode و برگردانده می‌شود.
7. اگر هر خطایی رخ دهد، `None` برمی‌گردد.

##### متد `_role_name`

هدف: mapping کردن roleهای Keycloak به roleهای Superset.

منطق:

1. roleهای `realm_access.roles` خوانده می‌شوند.
2. roleهای داخل `resource_access` برای همه clientها هم جمع‌آوری می‌شوند.
3. env `SUPERSET_KEYCLOAK_ADMIN_ROLES` خوانده و به set تبدیل می‌شود.
4. اگر roleهای کاربر با admin roleها intersection داشته باشد، نقش Superset برابر `Admin` می‌شود.
5. اگر admin role وجود نداشته باشد، مقدار `SUPERSET_KEYCLOAK_DEFAULT_ROLE` استفاده می‌شود.
6. مقدار پیش‌فرض role برای Operation برابر `Alpha` است.

#### توضیح envهای مرتبط با Token Exchange

در `.env.example` و `.env` این کلیدها اضافه شده‌اند:

```text
OPERATION_KEYCLOAK_BASE_URL=http://keycloak:8080
OPERATION_KEYCLOAK_REALM=bi-engine
OPERATION_KEYCLOAK_CLIENT_ID=superset-operation
OPERATION_KEYCLOAK_CLIENT_SECRET=local-dev-operation-secret
OPERATION_KEYCLOAK_TOKEN_EXCHANGE_ENABLED=true
OPERATION_KEYCLOAK_TOKEN_EXCHANGE_AUDIENCE=superset-operation
OPERATION_KEYCLOAK_TOKEN_EXCHANGE_REQUESTED_ISSUER=
OPERATION_KEYCLOAK_TOKEN_EXCHANGE_REQUESTED_TOKEN_TYPE=urn:ietf:params:oauth:token-type:access_token
```

هدف هر کدام:

- `OPERATION_KEYCLOAK_BASE_URL`: آدرس داخلی Keycloak عملیات از دید backend و Superset Operation.
- `OPERATION_KEYCLOAK_REALM`: realm عملیات.
- `OPERATION_KEYCLOAK_CLIENT_ID`: client عملیات برای Token Exchange و introspection.
- `OPERATION_KEYCLOAK_CLIENT_SECRET`: secret محرمانه client عملیات.
- `OPERATION_KEYCLOAK_TOKEN_EXCHANGE_ENABLED`: کنترل فعال بودن Token Exchange.
- `OPERATION_KEYCLOAK_TOKEN_EXCHANGE_AUDIENCE`: audience توکن خروجی.
- `OPERATION_KEYCLOAK_TOKEN_EXCHANGE_REQUESTED_ISSUER`: issuer هدف، اگر Keycloak نیاز داشته باشد.
- `OPERATION_KEYCLOAK_TOKEN_EXCHANGE_REQUESTED_TOKEN_TYPE`: نوع توکن خروجی.

#### توضیح تغییرات `docker-compose.yml`

در سرویس `super-app-backend`، envهای عملیات تزریق شده‌اند تا Go proxy بتواند با
Keycloak عملیات Token Exchange انجام دهد.

در سرویس `superset-operation`، envهای عملیات با نام‌های مورد انتظار security
manager تزریق شده‌اند:

```yaml
KEYCLOAK_BASE_URL: ${OPERATION_KEYCLOAK_BASE_URL:-...}
KEYCLOAK_REALM: ${OPERATION_KEYCLOAK_REALM:-...}
KEYCLOAK_CLIENT_ID: ${OPERATION_KEYCLOAK_CLIENT_ID:-superset-operation}
KEYCLOAK_CLIENT_SECRET: ${OPERATION_KEYCLOAK_CLIENT_SECRET:-local-dev-operation-secret}
```

دلیل این mapping این است که فایل Python داخل Superset از نام‌های عمومی
`KEYCLOAK_*` استفاده می‌کند، اما در container عملیات این نام‌ها عمدا به مقادیر
Keycloak عملیات اشاره می‌کنند.

#### توضیح تغییرات `infra/keycloak/realm-bi-engine.json`

برای local development یک client نمونه اضافه شده است:

```json
{
  "clientId": "superset-operation",
  "name": "Superset Operation",
  "enabled": true,
  "protocol": "openid-connect",
  "publicClient": false,
  "secret": "local-dev-operation-secret",
  "standardFlowEnabled": false,
  "directAccessGrantsEnabled": false,
  "serviceAccountsEnabled": true,
  "attributes": {
    "access.token.lifespan": "300"
  }
}
```

هدف فیلدها:

- `clientId`: نام client عملیات که backend و Superset Operation از آن استفاده می‌کنند.
- `publicClient=false`: یعنی client محرمانه است و secret دارد.
- `secret`: secret local برای توسعه.
- `standardFlowEnabled=false`: این client برای login مستقیم مرورگر نیست.
- `directAccessGrantsEnabled=false`: این client برای password grant نیست.
- `serviceAccountsEnabled=true`: برای سناریوهای service-level و introspection مفید است.
- `access.token.lifespan=300`: عمر کوتاه‌تر access token عملیات برای کاهش ریسک.

در Keycloak واقعی، علاوه بر این client، باید permissionهای Token Exchange نیز
تنظیم شوند. این permissionها معمولا از طریق policyهای Keycloak انجام می‌شوند و
ممکن است بسته به نسخه و تنظیمات Keycloak متفاوت باشند.

#### جمع‌بندی مسئولیت‌ها

- Browser فقط cookie داخلی Super App را نگه می‌دارد.
- Go backend مالک session، refresh token و Token Exchange است.
- MongoDB فقط hash session id و توکن‌های encrypted را ذخیره می‌کند.
- Keycloak عمومی login و هویت اولیه را صادر می‌کند.
- Keycloak عملیات توکن مخصوص Operation را صادر یا تأیید می‌کند.
- Superset Operation فقط توکنی را قبول می‌کند که introspection در Keycloak عملیات آن را active بداند.

## Local Zones

The POC runs two Superset containers from locally built images:

- `superset-public` uses `infra/superset/public/superset_config.py`
- `superset-operation` uses `infra/superset/operation/superset_config.py`

In this architecture, `superset-public` is treated as the public/display-side
Superset surface. It serves UI pages and static frontend assets only. It is not
the trusted service authority for operational JSON, query, chart-data, or API
requests.

Browser-facing requests under `/superset/public/*` still go through the Go
proxy. The Go proxy is path-aware:

```text
/superset/public/                      -> super-app-backend -> superset-public:8088
/superset/public/static/*              -> super-app-backend -> superset-public:8088
/superset/public/superset/welcome/*     -> super-app-backend -> superset-public:8088
/superset/public/api/*                  -> super-app-backend -> superset-operation:8088
/superset/public/superset/*_json*       -> super-app-backend -> superset-operation:8088
/superset/public/superset/results*      -> super-app-backend -> superset-operation:8088
/superset/public/superset/log*          -> super-app-backend -> superset-operation:8088
```

Operational requests are internally mapped from the public browser path to the
Operation application root:

```text
/superset/public/api/v1/chart/data
  -> super-app-backend
  -> /api/superset/operation/api/v1/chart/data
  -> superset-operation:8088
```

`superset-operation` is the service-side Superset instance. It owns operational
authentication, token introspection, DWH connectivity, and service execution.

Both configs are integration-time settings only. Superset code patches, map changes, Oracle drivers, OAuth changes, and image build logic remain in the Superset fork.
