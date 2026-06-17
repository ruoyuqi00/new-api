from __future__ import annotations

import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("kiro_web_adapter.py")
SPEC = importlib.util.spec_from_file_location("kiro_web_adapter", MODULE_PATH)
if SPEC is None or SPEC.loader is None:
    raise RuntimeError("failed to load kiro_web_adapter module")
kwa = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = kwa
SPEC.loader.exec_module(kwa)


class KiroWebAdapterRuntimeIdTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tempdir = tempfile.TemporaryDirectory()
        self.addCleanup(self.tempdir.cleanup)

    def make_client(self, credentials: list[dict[str, object]]) -> tuple[kwa.KiroWebClient, Path]:
        creds_path = Path(self.tempdir.name) / "credentials.json"
        creds_path.write_text(json.dumps(credentials, ensure_ascii=False, indent=2), encoding="utf-8")
        return kwa.KiroWebClient(str(creds_path)), creds_path

    def test_load_credentials_backfills_runtime_id(self) -> None:
        client, creds_path = self.make_client(
            [
                {
                    "email": "one@example.com",
                    "refreshToken": "refresh-1",
                    "availableModels": ["deepseek-3.2"],
                },
                {
                    "email": "two@example.com",
                    "refreshToken": "refresh-2",
                    "availableModels": ["deepseek-3.2"],
                    "runtime_id": "custom-runtime",
                },
            ]
        )

        usable, loaded = client.load_credentials("deepseek-3.2")

        self.assertEqual(len(usable), 2)
        self.assertIsInstance(loaded, list)
        written = json.loads(creds_path.read_text(encoding="utf-8"))
        self.assertEqual(
            written[0]["runtime_id"],
            kwa.credential_runtime_marker(written[0], 0),
        )
        self.assertEqual(written[1]["runtime_id"], "custom-runtime")

    def test_delete_credential_runtime_id_removes_only_matching_entry(self) -> None:
        client, creds_path = self.make_client(
            [
                {
                    "email": "one@example.com",
                    "refreshToken": "refresh-1",
                    "availableModels": ["deepseek-3.2"],
                    "runtime_id": "alpha",
                },
                {
                    "email": "two@example.com",
                    "refreshToken": "refresh-2",
                    "availableModels": ["deepseek-3.2"],
                    "runtime_id": "beta",
                },
            ]
        )
        client.session_cache["cached"] = object()

        removed = client.delete_credential_runtime_id("beta", "hard upstream failure: account locked")

        self.assertTrue(removed)
        self.assertEqual(client.session_cache, {})
        written = json.loads(creds_path.read_text(encoding="utf-8"))
        self.assertEqual(len(written), 1)
        self.assertEqual(written[0]["runtime_id"], "alpha")

    def test_hard_dead_reason_distinguishes_quota_text(self) -> None:
        self.assertEqual(
            kwa.hard_dead_reason("Your User ID is temporarily suspended. We detected unusual user activity."),
            "temporarily suspended",
        )
        self.assertIsNone(kwa.hard_dead_reason("monthly usage limit"))
        self.assertFalse(kwa.is_hard_dead_credential_text("monthly usage limit"))


if __name__ == "__main__":
    unittest.main()
