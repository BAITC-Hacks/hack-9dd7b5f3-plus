"""Greeting parsing and policy without provider calls."""
import unittest
from router import Router


class GreetingTests(unittest.TestCase):
    def test_greeting_is_a_valid_system_intent(self):
        router = Router(api_key="test-only")
        entries = router.parse("SYS_GREETING:100")
        result = router.decide(entries)
        self.assertEqual(result["status"], "greeting")
        self.assertEqual(result["predicted"], ["SYS_GREETING"])
        self.assertEqual(result["alternatives"], [])
