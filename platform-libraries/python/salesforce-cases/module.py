"""
Salesforce Cases — runeforge.salesforce_cases

Credentials (set as Credentials in the Variables tab):
  SALESFORCE_INSTANCE_URL     e.g. https://yourorg.my.salesforce.com
  SALESFORCE_ACCESS_TOKEN     long-lived / session token  <- simplest
  -- OR username-password OAuth flow --
  SALESFORCE_CLIENT_ID
  SALESFORCE_CLIENT_SECRET
  SALESFORCE_USERNAME
  SALESFORCE_PASSWORD
  SALESFORCE_SECURITY_TOKEN   (appended to password if required by your org)

Usage:
  from runeforge import salesforce_cases
  result = salesforce_cases.create_case({"Subject": "Login issue", "Status": "New"})
"""

from __future__ import annotations

import json
import os
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Optional


SF_API_VERSION = "v60.0"


def _auth() -> tuple[str, str]:
    instance_url = os.environ.get("SALESFORCE_INSTANCE_URL", "").rstrip("/")
    if not instance_url:
        raise ValueError("SALESFORCE_INSTANCE_URL is required")

    access_token = os.environ.get("SALESFORCE_ACCESS_TOKEN")
    if access_token:
        return instance_url, access_token

    client_id     = os.environ.get("SALESFORCE_CLIENT_ID", "")
    client_secret = os.environ.get("SALESFORCE_CLIENT_SECRET", "")
    username      = os.environ.get("SALESFORCE_USERNAME", "")
    password      = os.environ.get("SALESFORCE_PASSWORD", "")
    sec_token     = os.environ.get("SALESFORCE_SECURITY_TOKEN", "")

    if not all([client_id, client_secret, username]):
        raise ValueError(
            "Set SALESFORCE_ACCESS_TOKEN, or all of: SALESFORCE_CLIENT_ID, "
            "SALESFORCE_CLIENT_SECRET, SALESFORCE_USERNAME, SALESFORCE_PASSWORD"
        )

    payload = urllib.parse.urlencode({
        "grant_type":    "password",
        "client_id":     client_id,
        "client_secret": client_secret,
        "username":      username,
        "password":      password + sec_token,
    }).encode()

    req = urllib.request.Request(
        f"{instance_url}/services/oauth2/token",
        data=payload,
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        method="POST",
    )
    with urllib.request.urlopen(req) as resp:
        data = json.loads(resp.read())
    return data.get("instance_url", instance_url), data["access_token"]


def _sf_request(method: str, path: str, body: Optional[dict] = None) -> Any:
    instance_url, token = _auth()
    url = f"{instance_url}/services/data/{SF_API_VERSION}{path}"

    encoded = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(
        url,
        data=encoded,
        headers={
            "Authorization":  f"Bearer {token}",
            "Content-Type":   "application/json",
        },
        method=method,
    )
    try:
        with urllib.request.urlopen(req) as resp:
            if resp.status == 204:
                return None
            return json.loads(resp.read())
    except urllib.error.HTTPError as exc:
        raw = exc.read()
        try:
            err = json.loads(raw)
            msg = err[0]["message"] if isinstance(err, list) else err.get("message", str(exc))
        except Exception:
            msg = str(exc)
        raise RuntimeError(f"Salesforce {method} {path} failed ({exc.code}): {msg}") from exc


def create_case(fields: dict) -> dict:
    """
    Create a Salesforce Case.

    Common fields: Subject, Description, Status, Priority, Origin,
                   AccountId, ContactId, Type, Reason

    Returns: {"id": "...", "success": True, "errors": []}
    """
    return _sf_request("POST", "/sobjects/Case", fields)


def get_case(case_id: str) -> dict:
    """Fetch a Salesforce Case by its 15- or 18-character ID."""
    return _sf_request("GET", f"/sobjects/Case/{case_id}")


def update_case(case_id: str, fields: dict) -> None:
    """Update fields on an existing Case (PATCH — only changed fields needed)."""
    _sf_request("PATCH", f"/sobjects/Case/{case_id}", fields)


def delete_case(case_id: str) -> None:
    """Permanently delete a Case."""
    _sf_request("DELETE", f"/sobjects/Case/{case_id}")
