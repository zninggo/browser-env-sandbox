"""Smoke tests for bes Python SDK scheme resolution.

Zero-dependency (stdlib unittest): runnable with
    python sdk/python/test_scheme.py
or via
    python -m unittest sdk.python.test_scheme

Verifies the base_url contract — HTTPS addresses reach the wire as https://,
bare host:port stays http:// (backward compatibility).
"""

import os
import sys
import unittest

# Make the `bes` package importable when running this file directly.
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "bes"))
import bes  # noqa: E402


class TestNormalizeBaseURL(unittest.TestCase):
    def test_https_prefix_preserved(self):
        self.assertEqual(bes._normalize_base_url("https://example.com"), "https://example.com")

    def test_https_prefix_with_port_preserved(self):
        self.assertEqual(
            bes._normalize_base_url("https://bes.example.com:443"),
            "https://bes.example.com:443",
        )

    def test_http_prefix_preserved(self):
        self.assertEqual(
            bes._normalize_base_url("http://192.168.1.10:19821"),
            "http://192.168.1.10:19821",
        )

    def test_bare_host_port_defaults_to_http(self):
        self.assertEqual(bes._normalize_base_url("localhost:19821"), "http://localhost:19821")

    def test_bare_host_defaults_to_http(self):
        self.assertEqual(bes._normalize_base_url("bes.example.com"), "http://bes.example.com")


class TestConstructorBaseURL(unittest.TestCase):
    """Construct without a live server: __init__ calls _create_session, which
    will fail to connect, but base_url is set before that call. We therefore
    patch _create_session to a no-op so the scheme-resolution smoke check runs
    without network dependency.
    """

    def _make(self, server_addr):
        orig = bes.Sandbox._create_session
        bes.Sandbox._create_session = lambda self, **kw: None
        try:
            return bes.Sandbox(server_addr=server_addr)
        finally:
            bes.Sandbox._create_session = orig

    def test_https_address(self):
        s = self._make("https://example.com")
        self.assertEqual(s.base_url, "https://example.com")

    def test_bare_address_backward_compat(self):
        s = self._make("localhost:19821")
        self.assertEqual(s.base_url, "http://localhost:19821")


if __name__ == "__main__":
    unittest.main()
