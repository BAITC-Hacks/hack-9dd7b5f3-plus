"""HTTP contract tests; no API key or network provider required."""
import json
import threading
import unittest
import urllib.error
import urllib.request
from http.server import ThreadingHTTPServer
from unittest.mock import Mock

from router import RouterError
from server import make_handler, validate


class ServerTests(unittest.TestCase):
    def setUp(self):
        self.router = Mock(model="test-router")
        self.router.route.return_value = {"primary": "SC27", "predicted": ["SC27"], "status": "route"}
        self.server = ThreadingHTTPServer(("127.0.0.1", 0), make_handler(self.router))
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.url = f"http://127.0.0.1:{self.server.server_port}/api/route"

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()

    def request(self, payload):
        request = urllib.request.Request(self.url, json.dumps(payload).encode(), {"Content-Type": "application/json"})
        try:
            with urllib.request.urlopen(request) as response:
                return response.status, json.load(response)
        except urllib.error.HTTPError as error:
            with error:
                return error.code, json.load(error)

    def test_context_reaches_router(self):
        payload = {"text": "  А продлить?  ", "history": [{"text": "ОГПО", "scenario": "SC01"}],
                   "active": "SC27", "last_bot": "Ваш телефон?"}
        status, result = self.request(payload)
        self.assertEqual(status, 200)
        self.assertEqual(result["predicted"], ["SC27"])
        self.router.route.assert_called_once_with(**{**payload, "text": "А продлить?"})

    def test_invalid_requests_never_call_model(self):
        for payload in ([], {}, {"text": " "}, {"text": "x", "history": [1]}, {"text": "x", "active": []}):
            with self.subTest(payload=payload):
                self.assertEqual(self.request(payload)[0], 400)
        self.router.route.assert_not_called()

    def test_provider_errors_are_sanitized(self):
        self.router.route.side_effect = RouterError("private provider error")
        status, result = self.request({"text": "Полис"})
        self.assertEqual(status, 502)
        self.assertNotIn("private", result["error"])

    def test_large_text_is_rejected(self):
        with self.assertRaises(ValueError):
            validate({"text": "a" * 4001})


if __name__ == "__main__":
    unittest.main()
