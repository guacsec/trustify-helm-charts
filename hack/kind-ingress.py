#!/usr/bin/env python3
"""Configure Kind issuer DNS and verify HTTP routing through NGINX ingress.

Run from the repository root after ct installs the corresponding chart:
    python3 hack/kind-ingress.py sso          # before installing the application
    python3 hack/kind-ingress.py application  # after installing the application

Requires kubectl, curl, the kind-chart-testing context, and the NGINX controller
installed as release nginx in namespace nginx-ingress with host port 80 exposed.

The sso action maps sso.kind.test to NGINX's Service cluster IP in CoreDNS and
restarts CoreDNS when adding the mapping. This lets application pods reach the
same issuer hostname that Keycloak advertises. It then requires HTTP 200 from
the realm's OIDC discovery endpoint through ingress.

The application action requires HTTP 401 or 403 from an unauthenticated importer
API request through ingress. Both HTTP probes use curl's --resolve against
127.0.0.1, avoiding runner DNS changes. These supplement the Helm smoke tests,
which access application Services directly. Unexpected statuses fail the script.
"""

import argparse
import json
import subprocess


def http_status(host, address, path):
    return subprocess.check_output([
        "curl", "--noproxy", "*", "--silent", "--show-error",
        "--retry", "12", "--retry-connrefused", "--retry-delay", "5",
        "--max-time", "15", "--resolve", f"{host}:80:{address}",
        "--output", "/dev/null", "--write-out", "%{http_code}", f"http://{host}{path}",
    ], text=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["sso", "application"])
    parser.add_argument("--namespace", default="trustify-ci")
    args = parser.parse_args()
    if args.action == "sso":
        services = json.loads(subprocess.check_output([
            "kubectl", "--context", "kind-chart-testing", "-n", "nginx-ingress",
            "get", "services", "-l", "app.kubernetes.io/instance=nginx", "-o", "json",
        ], text=True))
        address = services["items"][0]["spec"]["clusterIP"]
        config = json.loads(subprocess.check_output([
            "kubectl", "--context", "kind-chart-testing", "-n", "kube-system", "get", "configmap", "coredns", "-o", "json",
        ], text=True))
        corefile = config["data"]["Corefile"]
        if "# trustify-kind-issuer" not in corefile:
            corefile = corefile.replace(".:53 {", f""".:53 {{
    # trustify-kind-issuer
    hosts {{
        {address} sso.kind.test
        fallthrough
    }}""", 1)
            subprocess.run([
                "kubectl", "--context", "kind-chart-testing", "-n", "kube-system", "patch", "configmap", "coredns",
                "--type=merge", "-p", json.dumps({"data": {"Corefile": corefile}}),
            ], check=True)
            subprocess.run(["kubectl", "--context", "kind-chart-testing", "-n", "kube-system", "rollout", "restart", "deployment/coredns"], check=True)
            subprocess.run(["kubectl", "--context", "kind-chart-testing", "-n", "kube-system", "rollout", "status", "deployment/coredns", "--timeout=90s"], check=True)
        status = http_status("sso.kind.test", "127.0.0.1", "/realms/trustify/.well-known/openid-configuration")
        if status != "200":
            raise RuntimeError(f"OIDC discovery through ingress returned HTTP {status}")
    else:
        status = http_status("server.kind.test", "127.0.0.1", "/api/v3/importer/ci-importer")
        if status not in ("401", "403"):
            raise RuntimeError(f"protected application API through ingress returned HTTP {status}")
    print(f"PASS: {args.action} ingress")


if __name__ == "__main__":
    main()
