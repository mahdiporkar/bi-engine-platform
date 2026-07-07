import os

from keycloak_proxy_security_manager import KeycloakProxySecurityManager

SECRET_KEY = os.environ.get("SUPERSET_SECRET_KEY", "change-me-local-superset-secret")
GUEST_TOKEN_JWT_SECRET = os.environ.get(
    "SUPERSET_GUEST_TOKEN_JWT_SECRET",
    "local-public-guest-token-secret-change-before-production",
)
SQLALCHEMY_DATABASE_URI = os.environ.get(
    "SQLALCHEMY_DATABASE_URI",
    "postgresql+psycopg2://superset:superset@superset-db:5432/superset",
)

ENABLE_PROXY_FIX = True
APPLICATION_ROOT = "/superset/public"
SESSION_COOKIE_SAMESITE = "None"
SESSION_COOKIE_SECURE = False
WTF_CSRF_ENABLED = False
RECAPTCHA_PUBLIC_KEY = ""
RECAPTCHA_PRIVATE_KEY = ""

FEATURE_FLAGS = {
    "DASHBOARD_RBAC": True,
}

TALISMAN_ENABLED = False
HTTP_HEADERS = {
    "X-Frame-Options": "ALLOWALL",
}

PUBLIC_ROLE_LIKE = "Gamma"
CUSTOM_SECURITY_MANAGER = KeycloakProxySecurityManager
AUTH_USER_REGISTRATION = True
AUTH_USER_REGISTRATION_ROLE = os.environ.get("SUPERSET_KEYCLOAK_DEFAULT_ROLE", "Gamma")
APP_NAME = "BI Engine Public Zone"
