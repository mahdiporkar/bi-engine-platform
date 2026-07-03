import json
import os
import urllib.parse
import urllib.request
from typing import Any, Optional

from flask import Request
from superset.security.manager import SupersetSecurityManager


class KeycloakProxySecurityManager(SupersetSecurityManager):
    def request_loader(self, request: Request) -> Optional[Any]:
        token = self._bearer_token(request)
        if not token:
            return None

        claims = self._introspect(token)
        if not claims or not claims.get("active"):
            return None

        username = claims.get("preferred_username") or claims.get("username") or claims.get("sub")
        if not username:
            return None

        email = claims.get("email") or f"{username}@local.invalid"
        first_name = claims.get("given_name") or username
        last_name = claims.get("family_name") or ""
        role = self.find_role(self._role_name(claims))

        user = self.find_user(username=username)
        if user:
            return user

        return self.add_user(
            username=username,
            first_name=first_name,
            last_name=last_name,
            email=email,
            role=role,
        )

    @staticmethod
    def _bearer_token(request: Request) -> str:
        header = request.headers.get("Authorization", "")
        if header.lower().startswith("bearer "):
            return header[7:].strip()
        return ""

    @staticmethod
    def _introspection_url() -> str:
        explicit_url = os.environ.get("KEYCLOAK_INTROSPECTION_URL")
        if explicit_url:
            return explicit_url

        base_url = os.environ.get("KEYCLOAK_BASE_URL", "http://keycloak:8080").rstrip("/")
        realm = os.environ.get("KEYCLOAK_REALM", "bi-engine")
        return f"{base_url}/realms/{realm}/protocol/openid-connect/token/introspect"

    def _introspect(self, token: str) -> Optional[dict[str, Any]]:
        client_id = os.environ.get("KEYCLOAK_CLIENT_ID", "super-app")
        client_secret = os.environ.get("KEYCLOAK_CLIENT_SECRET", "")
        payload = urllib.parse.urlencode(
            {
                "token": token,
                "client_id": client_id,
                "client_secret": client_secret,
            }
        ).encode("utf-8")

        request = urllib.request.Request(
            self._introspection_url(),
            data=payload,
            headers={"Content-Type": "application/x-www-form-urlencoded"},
            method="POST",
        )

        try:
            with urllib.request.urlopen(request, timeout=5) as response:
                return json.loads(response.read().decode("utf-8"))
        except Exception:
            return None

    @staticmethod
    def _role_name(claims: dict[str, Any]) -> str:
        roles = set(claims.get("realm_access", {}).get("roles", []))
        for resource in claims.get("resource_access", {}).values():
            roles.update(resource.get("roles", []))

        admin_roles = {
            role.strip()
            for role in os.environ.get(
                "SUPERSET_KEYCLOAK_ADMIN_ROLES",
                "bi-admin,superset-admin",
            ).split(",")
            if role.strip()
        }
        if roles.intersection(admin_roles):
            return "Admin"

        return os.environ.get("SUPERSET_KEYCLOAK_DEFAULT_ROLE", "Alpha")
