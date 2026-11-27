#!/usr/bin/env python3
"""Bootstrap issuer DNS or check application ingress after ct install."""

import argparse
import json
import subprocess
import time


def ingress_ip(namespace, name):
    deadline = time.monotonic() + 180
    while time.monotonic() < deadline:
        ingress = json.loads(subprocess.check_output([
            "kubectl", "--context", "kind-chart-testing", "-n", namespace, "get", "ingress", name, "-o", "json",
        ], text=True))
        addresses = ingress.get("status", {}).get("loadBalancer", {}).get("ingress", [])
        if addresses and addresses[0].get("ip"):
            return addresses[0]["ip"]
        time.sleep(2)
    raise RuntimeError(f"ingress {namespace}/{name} did not receive an address")


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
        address = ingress_ip(args.namespace, "infrastructure-keycloak")
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
        status = http_status("sso.kind.test", address, "/realms/trustify/.well-known/openid-configuration")
        if status != "200":
            raise RuntimeError(f"OIDC discovery through ingress returned HTTP {status}")
    else:
        address = ingress_ip(args.namespace, "server")
        status = http_status("server.kind.test", address, "/api/v3/importer/ci-importer")
        if status not in ("401", "403"):
            raise RuntimeError(f"protected application API through ingress returned HTTP {status}")
    print(f"PASS: {args.action} ingress")


if __name__ == "__main__":
    main()
