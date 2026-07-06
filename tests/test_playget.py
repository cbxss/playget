import json
import os
import subprocess
import sys
import time
import unittest
from pathlib import Path

import requests

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))

import playget


WITHINGS = "com.withings.wiscale2"
IATA = "org.iata.iataconnect"
COMPANION_SETUP = "android.software.companion_device_setup"


def candidate(extra=()):
    return playget.make_candidate("pixel_7a", list(extra), "test")


class OfflineSmoke(unittest.TestCase):
    def test_device_config_includes_runtime_overlay(self):
        cfg = playget.device_config(playget.BASE_PROFILE, [COMPANION_SETUP])
        self.assertIn(COMPANION_SETUP, list(cfg.systemAvailableFeature))
        self.assertNotIn(COMPANION_SETUP, playget.split_prop(playget.BASE_PROFILE, "features"))

    def test_checkin_request_embeds_device_config(self):
        req = playget.checkin_request(playget.BASE_PROFILE, [COMPANION_SETUP])
        self.assertEqual(req.locale, playget.LOCALE)
        self.assertIn(COMPANION_SETUP, list(req.deviceConfiguration.systemAvailableFeature))
        self.assertGreater(len(req.SerializeToString()), 100)

    def test_go_device_config_matches_python_oracle(self):
        py = subprocess.check_output([
            "uv", "run", "playget.py", "--dump-device-config-json",
            "--extra-feature", COMPANION_SETUP,
        ], cwd=ROOT, text=True)
        go = subprocess.check_output([
            "go", "run", "./cmd/playget", "--dump-device-config-json",
            "--extra-feature", COMPANION_SETUP,
        ], cwd=ROOT, text=True)
        self.assertEqual(json.loads(py), json.loads(go))


@unittest.skipUnless(os.environ.get("PLAYGET_LIVE") == "1", "set PLAYGET_LIVE=1 for live Play smoke tests")
class LiveSmoke(unittest.TestCase):
    _auth = None

    @classmethod
    def auth(cls):
        if cls._auth is None:
            last = None
            for attempt in range(5):
                try:
                    cls._auth = playget.dispenser()
                    break
                except requests.HTTPError as exc:
                    last = exc
                    status = exc.response.status_code if exc.response is not None else None
                    time.sleep(12 if status == 429 else 2)
            if cls._auth is None:
                raise last
        return cls._auth

    def with_retries(self, fn, attempts=3):
        last = None
        for attempt in range(attempts):
            try:
                return fn()
            except Exception as exc:
                last = exc
                if attempt + 1 == attempts:
                    break
                status = exc.response.status_code if isinstance(exc, requests.HTTPError) and exc.response is not None else None
                time.sleep(12 if status == 429 else 1.5)
        raise last

    def live_context(self, extra=()):
        email, token = self.auth()
        self.assertIn("@", email)
        sess = requests.Session()
        gsfid, ckt = playget.checkin(sess, playget.BASE_PROFILE, extra)
        self.assertTrue(gsfid)
        self.assertTrue(ckt)
        cft = playget.upload_device_config(sess, token, gsfid, ckt, playget.BASE_PROFILE, extra)
        self.assertTrue(cft)
        return sess, token, gsfid, ckt, cft

    def test_live_checkin_and_device_config_upload(self):
        self.with_retries(lambda: self.live_context())

    def test_live_withings_base_profile_reports_unavailable(self):
        def run():
            sess, token, gsfid, ckt, cft = self.live_context()
            with self.assertRaises(playget.PlayUnavailable) as ctx:
                playget.app_details(sess, token, gsfid, ckt, cft, playget.BASE_PROFILE, WITHINGS)
            self.assertEqual(ctx.exception.restriction, 9)

        self.with_retries(run)

    def test_live_withings_overlay_reaches_delivery_metadata(self):
        def run():
            sess = requests.Session()
            _, token = self.auth()
            title, vc, files, _cookies = playget.probe_download(sess, token, WITHINGS, 0, candidate([COMPANION_SETUP]))
            self.assertIn("Withings", title)
            self.assertGreater(vc, 0)
            self.assertTrue(any(name == "base.apk" for name, _url in files))

        self.with_retries(run)

    def test_live_iata_base_profile_reaches_delivery_metadata(self):
        def run():
            sess = requests.Session()
            _, token = self.auth()
            title, vc, files, _cookies = playget.probe_download(sess, token, IATA, 0, candidate())
            self.assertTrue(title)
            self.assertGreater(vc, 0)
            self.assertTrue(any(name == "base.apk" for name, _url in files))

        self.with_retries(run)
