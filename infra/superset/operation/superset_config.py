import os

SECRET_KEY = os.environ.get("SUPERSET_SECRET_KEY", "change-me-local-superset-secret")
GUEST_TOKEN_JWT_SECRET = os.environ.get(
    "SUPERSET_GUEST_TOKEN_JWT_SECRET",
    "local-operation-guest-token-secret-change-before-production",
)
SQLALCHEMY_DATABASE_URI = os.environ.get(
    "SQLALCHEMY_DATABASE_URI",
    "postgresql+psycopg2://superset:superset@superset-db:5432/superset",
)

ENABLE_PROXY_FIX = True
SESSION_COOKIE_SAMESITE = "None"
SESSION_COOKIE_SECURE = False
WTF_CSRF_ENABLED = False

FEATURE_FLAGS = {
    "EMBEDDED_SUPERSET": True,
    "DASHBOARD_RBAC": True,
}

TALISMAN_ENABLED = False
HTTP_HEADERS = {
    "X-Frame-Options": "ALLOWALL",
}

APP_NAME = "BI Engine Operation Zone"
