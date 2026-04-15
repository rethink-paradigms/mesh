import json
import time
import urllib.request
import urllib.error


class RouteManager:
    def __init__(self, caddy_admin_addr: str = "http://127.0.0.1:2019"):
        self._addr = caddy_admin_addr.rstrip("/")
        self._max_retries = 3
        self._retry_delay = 1

    def _request(self, method: str, path: str, data: dict = None) -> dict:
        url = f"{self._addr}{path}"
        body = json.dumps(data).encode("utf-8") if data else None
        last_error = None

        for attempt in range(self._max_retries):
            try:
                req = urllib.request.Request(url, data=body, method=method)
                req.add_header("Content-Type", "application/json")
                with urllib.request.urlopen(req) as resp:
                    raw = resp.read().decode("utf-8")
                    if raw:
                        return json.loads(raw)
                    return {}
            except (urllib.error.URLError, urllib.error.HTTPError, OSError) as e:
                last_error = e
                if attempt < self._max_retries - 1:
                    time.sleep(self._retry_delay)

        raise last_error

    def add_route(self, domain: str, upstream_host: str, upstream_port: int) -> bool:
        route = {
            "match": [{"host": [domain]}],
            "handle": [
                {
                    "handler": "reverse_proxy",
                    "upstreams": [{"dial": f"{upstream_host}:{upstream_port}"}],
                }
            ],
            "terminal": False,
        }
        try:
            routes_path = "/config/apps/http/servers/srv0/routes"
            existing = self._request("GET", routes_path)
            if not isinstance(existing, list):
                existing = []
            existing.append(route)
            self._request("PUT", routes_path, existing)
            return True
        except Exception:
            return False

    def remove_route(self, domain: str) -> bool:
        try:
            routes_path = "/config/apps/http/servers/srv0/routes"
            existing = self._request("GET", routes_path)
            if not isinstance(existing, list):
                return True
            filtered = [
                r
                for r in existing
                if not (
                    r.get("match")
                    and any(domain in m.get("host", []) for m in r["match"])
                )
            ]
            self._request("PUT", routes_path, filtered)
            return True
        except Exception:
            return False

    def update_route(self, domain: str, upstream_host: str, upstream_port: int) -> bool:
        removed = self.remove_route(domain)
        if not removed:
            return False
        return self.add_route(domain, upstream_host, upstream_port)

    def list_routes(self) -> list:
        try:
            routes_path = "/config/apps/http/servers/srv0/routes"
            result = self._request("GET", routes_path)
            if isinstance(result, list):
                return result
            return []
        except Exception:
            return []
