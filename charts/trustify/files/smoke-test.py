"""Functional checks run by `helm test`; never print tokens or response bodies."""

import json
import os
from pathlib import Path
from urllib.error import HTTPError
from urllib.parse import quote, urlencode
from urllib.request import Request, urlopen


def request(url, *, data=None, token=None, content_type=None, expected=(200,)):
    headers = {}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if content_type:
        headers["Content-Type"] = content_type
    try:
        with urlopen(Request(url, data=data, headers=headers), timeout=30) as response:
            status, body = response.status, response.read()
    except HTTPError as error:
        status, body = error.code, error.read()
    if status not in expected:
        raise RuntimeError(f"{url}: expected {expected}, received HTTP {status}")
    return body


def main():
    base = os.environ["API_URL"]
    importer_url = f"{base}/api/v3/importer/ci-importer"
    request(importer_url, expected=(401, 403))
    form = urlencode({
        "grant_type": "client_credentials",
        "client_id": os.environ["CLIENT_ID"],
        "client_secret": os.environ["CLIENT_SECRET"],
    }).encode()
    token = json.loads(request(
        f"{os.environ['ISSUER_URL']}/protocol/openid-connect/token",
        data=form, content_type="application/x-www-form-urlencoded",
    ))["access_token"]
    importer = json.loads(request(importer_url, token=token))
    configuration = importer["configuration"]["sbom"]
    assert configuration["description"] == "Kind smoke importer", "seeded importer differs"
    assert configuration["disabled"] is True, "external importer must stay disabled"

    document = Path("/tests/sbom.json").read_bytes()
    uploaded = json.loads(request(
        f"{base}/api/v3/sbom", data=document, token=token,
        content_type="application/json", expected=(201,),
    ))
    key = quote(uploaded["id"], safe="")
    request(f"{base}/api/v3/sbom/{key}", token=token)
    downloaded = request(f"{base}/api/v3/sbom/{key}/download", token=token)
    assert json.loads(downloaded) == json.loads(document), "stored SBOM differs"
    print("PASS: authentication, seeded importer, SBOM upload, lookup and download")


if __name__ == "__main__":
    main()
