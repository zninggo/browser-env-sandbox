"""
browser-env-sandbox Python SDK

Connects to bes-server via JSON-RPC over HTTP.
No gRPC dependency needed.

Usage:
    from bes import Sandbox

    sandbox = Sandbox(browser="chrome", os="windows")
    print(sandbox.eval("navigator.userAgent"))
    sandbox.load_script(".js", open(".js").read())
    result = sandbox.call("sign", "param1", "param2")
    print(sandbox.fingerprint)
    sandbox.close()

    # Context manager
    with Sandbox() as s:
        print(s.eval("navigator.platform"))
"""

import json
import urllib.request
import urllib.error
from typing import Optional, Dict, List, Any


class BESError(Exception):
    """browser-env-sandbox SDK error."""
    pass


def _normalize_base_url(server_addr: str) -> str:
    """Keep backward compatibility: a bare host:port stays http://, while an
    address that already carries an http:// or https:// scheme is used verbatim
    so HTTPS endpoints can be reached.
    """
    if server_addr.startswith("http://") or server_addr.startswith("https://"):
        return server_addr
    return f"http://{server_addr}"


class Sandbox:
    """A browser environment sandbox session."""

    def __init__(
        self,
        server_addr: str = "localhost:19821",
        browser: str = "chrome",
        os: str = "windows",
        seed: int = 0,
        location: str = "https://example.com/",
        cookies: Optional[Dict[str, str]] = None,
        proxy: str = "",
        net_mode: str = "live",
        recording: str = "",
    ):
        """
        Create a sandbox session.

        Args:
            server_addr: bes-server address. A bare "host:port" resolves to
                http:// (the historical default for local bes-server); pass an
                "http://" or "https://" prefixed address to reach a TLS fronted
                instance.
            browser: Browser type ("chrome", "firefox", "safari")
            os: Operating system ("windows", "macos", "linux", "android")
            seed: Fingerprint seed (0 = random)
            location: document.URL
            cookies: Initial cookies
            proxy: Proxy URL
            net_mode: "replay" or "live"
            recording: Path to recording file for replay mode
        """
        self.server_addr = server_addr
        self.base_url = _normalize_base_url(server_addr)
        self.session_id: Optional[str] = None
        self._create_session(
            browser=browser, os=os, seed=seed, location=location,
            cookies=cookies or {}, proxy=proxy, net_mode=net_mode, recording=recording,
        )

    def _post(self, path: str, data: dict) -> dict:
        """Send a JSON POST request to the server."""
        url = f"{self.base_url}{path}"
        body = json.dumps(data).encode("utf-8")
        req = urllib.request.Request(
            url, data=body,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                return json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            err_body = e.read().decode("utf-8")
            raise BESError(f"HTTP {e.code}: {err_body}") from e
        except urllib.error.URLError as e:
            raise BESError(f"Connection failed: {e}") from e

    def _get(self, path: str) -> dict:
        """Send a GET request to the server."""
        url = f"{self.base_url}{path}"
        req = urllib.request.Request(url, method="GET")
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                return json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            raise BESError(f"HTTP {e.code}: {e.read().decode('utf-8')}") from e
        except urllib.error.URLError as e:
            raise BESError(f"Connection failed: {e}") from e

    def _delete(self, path: str) -> dict:
        """Send a DELETE request to the server."""
        url = f"{self.base_url}{path}"
        req = urllib.request.Request(url, method="DELETE")
        try:
            with urllib.request.urlopen(req, timeout=30) as resp:
                return json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            raise BESError(f"HTTP {e.code}") from e

    def _create_session(self, **kwargs):
        """Create a session on the server."""
        resp = self._post("/api/session", kwargs)
        if "error" in resp:
            raise BESError(resp["error"])
        self.session_id = resp.get("session_id")
        if not self.session_id:
            raise BESError("No session_id in response")

    def eval(self, code: str) -> str:
        """Execute JavaScript in the sandbox and return the result."""
        if not self.session_id:
            raise BESError("Session not created")
        resp = self._post(f"/api/session/{self.session_id}/eval", {"code": code})
        if "error" in resp and resp["error"]:
            raise BESError(resp["error"])
        return resp.get("result", "")

    def load_script(self, name: str, content: str = None) -> None:
        """Load and execute a script.

        If content is None, name is treated as a file path.
        """
        if not self.session_id:
            raise BESError("Session not created")
        if content is None:
            with open(name, "r") as f:
                content = f.read()
        resp = self._post(f"/api/session/{self.session_id}/script", {
            "name": name, "content": content,
        })
        if "error" in resp and resp["error"]:
            raise BESError(resp["error"])

    def call(self, function_name: str, *args) -> str:
        """Call a global function by name with string arguments."""
        if not self.session_id:
            raise BESError("Session not created")
        resp = self._post(f"/api/session/{self.session_id}/call", {
            "function": function_name, "args": list(args),
        })
        if "error" in resp and resp["error"]:
            raise BESError(resp["error"])
        return resp.get("result", "")

    @property
    def fingerprint(self) -> dict:
        """Get the session's fingerprint."""
        if not self.session_id:
            raise BESError("Session not created")
        return self._get(f"/api/session/{self.session_id}/fingerprint")

    @property
    def cookies(self) -> str:
        """Get the session's cookies as a string."""
        if not self.session_id:
            raise BESError("Session not created")
        resp = self._get(f"/api/session/{self.session_id}/cookies")
        return resp.get("cookies", "")

    def set_cookie(self, name: str, value: str) -> None:
        """Set a cookie in the session."""
        if not self.session_id:
            raise BESError("Session not created")
        self._post(f"/api/session/{self.session_id}/cookies", {"name": name, "value": value})

    def close(self):
        """Close the session and release resources."""
        if self.session_id:
            try:
                self._delete(f"/api/session/{self.session_id}")
            except BESError:
                pass  # Server might be gone
            self.session_id = None

    def stream_console(self, callback):
        """Stream console messages via SSE. Calls callback(message_dict) for each message.

        This is a blocking call — run in a separate thread if needed.
        """
        import urllib.request
        url = f"{self.base_url}/api/session/{self.session_id}/stream/console"
        try:
            req = urllib.request.Request(url, method="GET")
            resp = urllib.request.urlopen(req, timeout=None)
            for line in resp:
                line = line.decode("utf-8").strip()
                if line.startswith("data: "):
                    data = line[6:]
                    if data == "[DONE]":
                        break
                    try:
                        callback(json.loads(data))
                    except json.JSONDecodeError:
                        pass
        except Exception as e:
            raise BESError(f"Stream error: {e}")

    def __enter__(self):
        return self

    def __exit__(self, *args):
        self.close()
