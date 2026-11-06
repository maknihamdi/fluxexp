#!/usr/bin/env python3
"""Serves internal/ui/web with a fake /api, to drive the portal's namespace
selector in a browser without a cluster. flux is refused; every API call is logged
to stdout, so "no call" can be observed. Usage: fake-portal.py [port]"""
import json, os, sys
from http.server import ThreadingHTTPServer, SimpleHTTPRequestHandler
from urllib.parse import urlparse, parse_qs

WEB = os.path.join(os.path.dirname(__file__), "..", "internal", "ui", "web")
KS = "kustomize.toolkit.fluxcd.io/v1|Kustomization"
HR = "helm.toolkit.fluxcd.io/v2|HelmRelease"

def root(t, ns, name):
    return {"ref": {"domain": "kubernetes", "type": t, "namespace": ns, "name": name,
                    "display": f"{t.split('|')[1]} {ns}/{name}"},
            "health": "healthy", "interval": "5m", "message": "ok"}

ROOTS = {
    "team-a": ([root(HR, "team-a", "web")], None),
    "team-b": ([root(KS, "team-b", "apps"), root(HR, "team-b", "api")], None),
    "partial": ([root(HR, "partial", "web")], ["Kustomization"]),
    "empty": ([], None),
}

class H(SimpleHTTPRequestHandler):
    def __init__(self, *a, **k): super().__init__(*a, directory=WEB, **k)
    def log_message(self, *a): pass
    def j(self, code, body):
        b = json.dumps(body).encode()
        self.send_response(code); self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
    def do_GET(self):
        u = urlparse(self.path); q = {k: v[0] for k, v in parse_qs(u.query).items()}
        if not u.path.startswith("/api/"):
            return super().do_GET()
        print(self.path, flush=True)
        if u.path == "/api/contexts":
            return self.j(200, {"contexts": [{"name": "dev", "current": True}, {"name": "prod"}]})
        if u.path == "/api/namespaces":
            names = ["empty", "flux", "kube-system", "partial", "team-a", "team-b"]
            return self.j(200, {"namespaces": names if q.get("context") == "dev" else ["prod-ns"]})
        if u.path == "/api/roots":
            ns = q.get("namespace", "flux")
            if ns in ROOTS:
                r, un = ROOTS[ns]
                return self.j(200, {"roots": r, "unreadable": un})
            return self.j(502, {"error": f'listing Kustomization: kustomizations is forbidden: User "me" cannot list resource "kustomizations" in the namespace "{ns}"'})
        if u.path == "/api/expand":
            t, ns, n = q["type"], q.get("ns", ""), q["name"]
            return self.j(200, {"ref": {"domain": "kubernetes", "type": t, "namespace": ns, "name": n,
                                        "display": f"{t.split('|')[1]} {ns}/{n}"},
                                "health": "healthy", "expandable": True, "context": q.get("context", ""), "children": []})
        self.j(404, {"error": "nope"})

ThreadingHTTPServer(("127.0.0.1", int(sys.argv[1]) if len(sys.argv) > 1 else 8799), H).serve_forever()
