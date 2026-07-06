#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.9"
# dependencies = ["requests>=2.31", "protobuf>=5"]
# ///
"""
playget - download Google Play apps locally.

Pure Python: anonymous Aurora-dispenser credentials + Google Play's protocol
(checkin -> uploadDeviceConfig -> details -> purchase -> delivery), reimplemented
from Aurora's gplayapi. No JVM, no emulator, no token, no cloud.
Must run from a residential IP (Google blocks Play login from datacenter IPs).

Usage:
    uv run playget.py <package> [--version <versionCode>] [--out <dir>]

Examples:
    uv run playget.py com.anthropic.claude
    uv run playget.py com.anthropic.claude --version 26020937
"""
import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import time

import requests

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import googleplay_pb2 as gp  # clean, self-contained Play protobuf bindings

BASE = "https://android.clients.google.com"
DISPENSER = "https://auroraoss.com/api/auth"
DISPENSER_UA = "com.aurora.store-4.7.4"
LOCALE = "en_US"
DEFAULT_PROFILE = "pixel_7a"
CACHE_VERSION = 1

# Finsky header blobs (from a current Aurora gplayapi release). If Google ever starts
# rejecting these, refresh them from gplayapi's HeaderProvider.kt source.
DFE_TARGETS = ("CAESN/qigQYC2AMBFfUbyA7SM5Ij/CvfBoIDgxHqGP8R3xzIBvoQtBKFDZ4HAY4FrwSVMasHBO0O2Q8a"
    "kgYRAQECAQO7AQEpKZ0CnwECAwRrAQYBr9PPAoK7sQMBAQMCBAkIDAgBAwEDBAICBAUZEgMEBAMLAQEBBQEBAcYBARYE"
    "D+cBfS8CHQEKkAEMMxcBIQoUDwYHIjd3DQ4MFk0JWGYZEREYAQOLAYEBFDMIEYMBAgICAgICOxkCD18LGQKEAcgDBIQB"
    "AgGLARkYCy8oBTJlBCUocxQn0QUBDkkGxgNZQq0BZSbeAmIDgAEBOgGtAaMCDAOQAZ4BBIEBKUtQUYYBQscDDxPSARA1"
    "oAEHAWmnAsMB2wFyywGLAxol+wImlwOOA80CtwN26A0WjwJVbQEJPAH+BRDeAfkHK/ABASEBCSAaHQemAzkaRiu2Ad8B"
    "dXeiAwEBGBUBBN4LEIABK4gB2AFLfwECAdoENq0CkQGMBsIBiQEtiwGgA1zyAUQ4uwS8AwhsvgPyAcEDF27vApsBHaIC"
    "Ghl3GSKxAR8MC6cBAgItmQYG9QIeywLvAeYBDArLAh8HASI4ELICDVmVBgsY/gHWARtcAsMBpALiAdsBA7QBpAJmIArp"
    "Byn0AyAKBwHTARIHAX8D+AMBcRIBBbEDmwUBMacCHAciNp0BAQF0OgQLJDuSAh54kwFSP0eeAQQ4M5EBQgMEmwFXywFo"
    "0gFyWwMcapQBBugBPUW2AVgBKmy3AR6PAbMBGQxrUJECvQR+8gFoWDsYgQNwRSczBRXQAgtRswEW0ALMAREYAUEBIG6y"
    "ATYCRE8OxgER8gMBvQEDRkwLc8MBTwHZAUOnAXiiBakDIbYBNNcCIUmuArIBSakBrgFHKs0EgwV/G3AD0wE6LgECtQJ4"
    "xQFwFbUCjQPkBS6vAQqEAUZF3QIM9wEhCoYCQhXsBCyZArQDugIziALWAdIBlQHwBdUErQE6qQaSA4EEIvYBHir9AQVL"
    "mgMCApsCKAwHuwgrENsBAjNYswEVmgIt7QJnN4wDEnta+wGfAcUBxgEtEFXQAQWdAUAeBcwBAQM7rAEJATJ0LENrdh73"
    "A6UBhAE+qwEeASxLZUMhDREuH0CGARbd7K0GlQo")
DFE_PHENOTYPE = ("H4sIAAAAAAAAAB3OO3KjMAAA0KRNuWXukBkBQkAJ2MhgAZb5u2GCwQZbCH_EJ77QHmgvtDtbv-Z9_H63z"
    "XXU0NVPB1odlyGy7751Q3CitlPDvFd8lxhz3tpNmz7P92CFw73zdHU2Ie0Ad2kmR8lxhiErTFLt3RPGfJQHSDy7Clw10b"
    "g8kqf2owLokN4SecJTLoSwBnzQSd652_MOf2d1vKBNVedzg4ciPoLz2mQ8efGAgYeLou-l-PXn_7Sna1MfhHuySxt-4es"
    "ulEDp8Sbq54CPPKjpANW-lkU2IZ0F92LBI-ukCKSptqeq1eXU96LD9nZfhKHdtjSWwJqUm_2r6pMHOxk01saVanmNopjX"
    "3YxQafC4iC6T55aRbC8nTI98AF_kItIQAJb5EQxnKTO7TZDWnr01HVPxelb9A2OWX6poidMWl16K54kcu_jhXw-JSBQkV"
    "cD_fPsLSZu6joIBAAA")

AUTO_FEATURE_OVERLAYS = (
    ("android.software.companion_device_setup",),
)
FEATURE_RE = re.compile(r"^\s*uses-feature: name='([^']+)'", re.MULTILINE)


class PlayUnavailable(RuntimeError):
    def __init__(self, title, restriction):
        self.title = title
        self.restriction = restriction
        super().__init__("%s unavailable (availability restriction=%s)" % (title, restriction))


class EmptyDelivery(RuntimeError):
    pass


def load_device(path):
    d = {}
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                d[k.strip().lower()] = v.strip()
    return d


BASE_PROFILE = load_device(os.path.join(HERE, "device.properties"))
PROFILES = {DEFAULT_PROFILE: BASE_PROFILE}


def dv(profile, key):
    return profile.get(key.lower(), "")


def split_prop(profile, key):
    return [x.strip() for x in dv(profile, key).split(",") if x.strip()]


def unique(items):
    out, seen = [], set()
    for item in items:
        if item and item not in seen:
            out.append(item)
            seen.add(item)
    return out


def merged_features(profile, extra_features=()):
    return unique(split_prop(profile, "features") + list(extra_features))


def profile_label(name, extra_features):
    if not extra_features:
        return name
    return "%s+%s" % (name, "+".join(extra_features))


def cache_path():
    if os.environ.get("PLAYGET_CACHE"):
        return os.environ["PLAYGET_CACHE"]
    root = os.environ.get("XDG_CACHE_HOME", os.path.join(os.path.expanduser("~"), ".cache"))
    return os.path.join(root, "playget", "profile-cache.json")


def read_cache(enabled):
    if not enabled:
        return {"version": CACHE_VERSION, "packages": {}}
    try:
        with open(cache_path(), encoding="utf-8") as fh:
            data = json.load(fh)
        if data.get("version") == CACHE_VERSION and isinstance(data.get("packages"), dict):
            return data
    except (OSError, ValueError):
        pass
    return {"version": CACHE_VERSION, "packages": {}}


def write_cache(cache, enabled):
    if not enabled:
        return
    path = cache_path()
    os.makedirs(os.path.dirname(path), exist_ok=True)
    tmp = path + ".tmp"
    with open(tmp, "w", encoding="utf-8") as fh:
        json.dump(cache, fh, indent=2, sort_keys=True)
        fh.write("\n")
    os.replace(tmp, path)


def make_candidate(profile_name, extra_features, source):
    if profile_name not in PROFILES:
        raise SystemExit("unknown profile %r (known: %s)" %
                         (profile_name, ", ".join(sorted(PROFILES))))
    return {
        "name": profile_name,
        "profile": PROFILES[profile_name],
        "extra": tuple(unique(extra_features)),
        "source": source,
    }


def profile_candidates(pkg, profile_name, cli_extra_features, cache, use_cache):
    if profile_name != "auto":
        return [make_candidate(profile_name, cli_extra_features, "cli")]

    candidates, seen = [], set()

    def add(candidate):
        key = (candidate["name"], candidate["extra"])
        if key not in seen:
            candidates.append(candidate)
            seen.add(key)

    if use_cache:
        entry = cache.get("packages", {}).get(pkg)
        if entry:
            cached_features = entry.get("extra_features") or []
            add(make_candidate(entry.get("profile") or DEFAULT_PROFILE,
                               list(cached_features) + list(cli_extra_features),
                               "cache"))

    add(make_candidate(DEFAULT_PROFILE, cli_extra_features, "base"))
    for overlay in AUTO_FEATURE_OVERLAYS:
        add(make_candidate(DEFAULT_PROFILE, list(overlay) + list(cli_extra_features), "auto"))
    return candidates


def user_agent(profile):
    return ("Android-Finsky/{vs} (api=3,versionCode={vc},sdk={sdk},device={dev},hardware={hw},"
            "product={pr},platformVersionRelease={pv},model={md},buildId={bid},isWideScreen=0,"
            "supportedAbis={abis})").format(
        vs=dv(profile, "vending.versionstring"), vc=dv(profile, "vending.version"),
        sdk=dv(profile, "build.version.sdk_int"), dev=dv(profile, "build.device"),
        hw=dv(profile, "build.hardware"), pr=dv(profile, "build.product"),
        pv=dv(profile, "build.version.release"), md=dv(profile, "build.model"),
        bid=dv(profile, "build.id"), abis=dv(profile, "platforms").replace(",", ";"))


def android_build(profile):
    b = gp.AndroidBuildProto()
    b.id = dv(profile, "build.fingerprint"); b.product = dv(profile, "build.hardware")
    b.carrier = dv(profile, "build.brand"); b.radio = dv(profile, "build.radio")
    b.bootloader = dv(profile, "build.bootloader"); b.device = dv(profile, "build.device")
    b.sdkVersion = int(dv(profile, "build.version.sdk_int")); b.model = dv(profile, "build.model")
    b.manufacturer = dv(profile, "build.manufacturer"); b.buildProduct = dv(profile, "build.product")
    b.client = dv(profile, "client"); b.otaInstalled = False; b.timestamp = int(time.time())
    b.googleServices = int(dv(profile, "gsf.version"))
    return b


def device_config(profile, extra_features=()):
    c = gp.DeviceConfigurationProto()
    c.touchScreen = int(dv(profile, "touchscreen")); c.keyboard = int(dv(profile, "keyboard"))
    c.navigation = int(dv(profile, "navigation")); c.screenLayout = int(dv(profile, "screenlayout"))
    c.hasHardKeyboard = dv(profile, "hashardkeyboard") == "true"
    c.hasFiveWayNavigation = dv(profile, "hasfivewaynavigation") == "true"
    c.screenDensity = int(dv(profile, "screen.density")); c.screenWidth = int(dv(profile, "screen.width"))
    c.screenHeight = int(dv(profile, "screen.height")); c.glEsVersion = int(dv(profile, "gl.version"))
    for x in split_prop(profile, "platforms"): c.nativePlatform.append(x)
    for x in split_prop(profile, "sharedlibraries"): c.systemSharedLibrary.append(x)
    for x in merged_features(profile, extra_features): c.systemAvailableFeature.append(x)
    for x in split_prop(profile, "locales"): c.systemSupportedLocale.append(x)
    for x in split_prop(profile, "gl.extensions"): c.glExtension.append(x)
    return c


def checkin_request(profile, extra_features=()):
    r = gp.AndroidCheckinRequest()
    r.id = 0
    r.checkin.build.CopyFrom(android_build(profile))
    r.checkin.lastCheckinMsec = 0
    r.checkin.cellOperator = dv(profile, "celloperator")
    r.checkin.simOperator = dv(profile, "simoperator")
    r.checkin.roaming = dv(profile, "roaming")
    r.checkin.userNumber = 0
    r.locale = LOCALE
    r.timeZone = dv(profile, "timezone") or "America/Los_Angeles"
    r.version = 3
    r.deviceConfiguration.CopyFrom(device_config(profile, extra_features))
    r.fragment = 0
    return r


def fdfe_headers(profile, token, gsfid, checkin_token="", config_token=""):
    h = {
        "Authorization": "Bearer " + token,
        "User-Agent": user_agent(profile),
        "X-DFE-Device-Id": gsfid,
        "Accept-Language": LOCALE.replace("_", "-"),
        "X-DFE-Encoded-Targets": DFE_TARGETS,
        "X-DFE-Phenotype": DFE_PHENOTYPE,
        "X-DFE-Client-Id": "am-android-google",
        "X-DFE-Network-Type": "4",
        "X-DFE-Content-Filters": "",
        "X-Limit-Ad-Tracking-Enabled": "false",
        "X-Ad-Id": "",
        "X-DFE-UserLanguages": LOCALE,
        "X-DFE-Request-Params": "timeoutMs=4000",
        "X-DFE-MCCMNC": dv(profile, "simoperator"),
    }
    if checkin_token:
        h["X-DFE-Device-Checkin-Consistency-Token"] = checkin_token
    if config_token:
        h["X-DFE-Device-Config-Token"] = config_token
    return h


def wrapper(content):
    w = gp.ResponseWrapper()
    w.ParseFromString(content)
    return w


def checkin(sess, profile, extra_features=()):
    req = checkin_request(profile, extra_features)
    h = {"app": "com.google.android.gms",
         "User-Agent": "GoogleAuth/1.4 (%s %s)" % (dv(profile, "build.device"), dv(profile, "build.id")),
         "Content-Type": "application/x-protobuffer",
         "Host": "android.clients.google.com"}
    r = sess.post(BASE + "/checkin", data=req.SerializeToString(), headers=h, timeout=40)
    r.raise_for_status()
    resp = gp.AndroidCheckinResponse()
    resp.ParseFromString(r.content)
    return "%x" % resp.androidId, resp.deviceCheckinConsistencyToken


def upload_device_config(sess, token, gsfid, ckt, profile, extra_features=()):
    up = gp.UploadDeviceConfigRequest()
    up.deviceConfiguration.CopyFrom(device_config(profile, extra_features))
    r = sess.post(BASE + "/fdfe/uploadDeviceConfig", data=up.SerializeToString(),
                  headers=fdfe_headers(profile, token, gsfid, ckt), timeout=40)
    r.raise_for_status()
    return wrapper(r.content).payload.uploadDeviceConfigResponse.uploadDeviceConfigToken


def app_details(sess, token, gsfid, ckt, cft, profile, pkg):
    r = sess.get(BASE + "/fdfe/details", params={"doc": pkg},
                 headers=fdfe_headers(profile, token, gsfid, ckt, cft), timeout=40)
    r.raise_for_status()
    doc = wrapper(r.content).payload.detailsResponse.docV2
    vc = doc.details.appDetails.versionCode
    if not vc:
        restriction = doc.availability.restriction if doc.HasField("availability") else "unknown"
        raise PlayUnavailable(doc.title or pkg, restriction)
    return vc, (doc.title or pkg)


def delivery_token(sess, token, gsfid, ckt, cft, profile, pkg, vc, ot=1):
    r = sess.post(BASE + "/fdfe/purchase", params={"ot": str(ot), "doc": pkg, "vc": str(vc)},
                  data="", headers=fdfe_headers(profile, token, gsfid, ckt, cft), timeout=40)
    r.raise_for_status()
    return wrapper(r.content).payload.buyResponse.downloadToken


def delivery(sess, token, gsfid, ckt, cft, profile, pkg, vc, dtok, ot=1):
    params = {"ot": str(ot), "doc": pkg, "vc": str(vc)}
    if dtok:
        params["dtok"] = dtok
    r = sess.get(BASE + "/fdfe/delivery", params=params,
                 headers=fdfe_headers(profile, token, gsfid, ckt, cft), timeout=40)
    r.raise_for_status()
    data = wrapper(r.content).payload.deliveryResponse.appDeliveryData
    files = []
    if data.downloadUrl:
        files.append(("base.apk", data.downloadUrl))
    for s in data.split:
        files.append((s.name if s.name.endswith(".apk") else "%s.apk" % s.name, s.downloadUrl))
    cookies = {c.name: c.value for c in data.downloadAuthCookie}
    return files, cookies


def dispenser():
    r = requests.get(DISPENSER, headers={"User-Agent": DISPENSER_UA, "Accept": "application/json"},
                     timeout=25)
    r.raise_for_status()
    j = r.json()
    return j["email"], j["auth"]


def download(url, cookies, dest):
    with requests.get(url, cookies=cookies, stream=True, timeout=180) as r:
        r.raise_for_status()
        with open(dest, "wb") as fh:
            for chunk in r.iter_content(1 << 16):
                fh.write(chunk)
    return os.path.getsize(dest)


def probe_download(sess, token, pkg, version, candidate):
    profile = candidate["profile"]
    extra = candidate["extra"]
    gsfid, ckt = checkin(sess, profile, extra)
    cft = upload_device_config(sess, token, gsfid, ckt, profile, extra)
    print("[*] auth ok. gsfId=%s" % gsfid, file=sys.stderr)

    latest_vc, title = app_details(sess, token, gsfid, ckt, cft, profile, pkg)
    vc = version or latest_vc
    print("[*] %s  versionCode=%s" % (title, vc), file=sys.stderr)
    dtok = delivery_token(sess, token, gsfid, ckt, cft, profile, pkg, vc)
    files, cookies = delivery(sess, token, gsfid, ckt, cft, profile, pkg, vc, dtok)
    if not files:
        raise EmptyDelivery("delivery returned no files")
    return title, vc, files, cookies


def learn_required_features(base_apk):
    if not base_apk or not shutil.which("aapt"):
        return []
    try:
        result = subprocess.run(["aapt", "dump", "badging", base_apk],
                                check=True, stdout=subprocess.PIPE,
                                stderr=subprocess.PIPE, text=True)
    except (OSError, subprocess.CalledProcessError):
        return []
    return sorted(set(FEATURE_RE.findall(result.stdout)))


def update_profile_cache(cache, pkg, candidate, downloaded, enabled):
    profile = candidate["profile"]
    extra = list(candidate["extra"])
    base_apk = next((dest for name, dest in downloaded if name == "base.apk"), None)
    missing = [f for f in learn_required_features(base_apk) if f not in merged_features(profile)]
    extra = unique(extra + missing)
    cache.setdefault("packages", {})[pkg] = {
        "profile": candidate["name"],
        "extra_features": extra,
        "updated": int(time.time()),
    }
    write_cache(cache, enabled)
    if extra:
        print("[*] cached profile %s for %s" %
              (profile_label(candidate["name"], tuple(extra)), pkg), file=sys.stderr)


def fetch(pkg, version, out_dir, retries=4, profile_name="auto", extra_features=(), use_cache=True):
    cache = read_cache(use_cache)
    candidates = profile_candidates(pkg, profile_name, tuple(extra_features), cache, use_cache)
    last = None
    for attempt in range(1, retries + 1):
        try:
            email, token = dispenser()
            print("[*] dispenser: %s" % email, file=sys.stderr)
            errors = []
            for candidate in candidates:
                label = profile_label(candidate["name"], candidate["extra"])
                print("[*] trying profile: %s" % label, file=sys.stderr)
                try:
                    sess = requests.Session()
                    _title, _vc, files, cookies = probe_download(sess, token, pkg, version, candidate)
                    print("[*] files: %d" % len(files), file=sys.stderr)
                    os.makedirs(out_dir, exist_ok=True)
                    downloaded = []
                    for name, url in files:
                        dest = os.path.join(out_dir, name)
                        got = download(url, cookies, dest)
                        downloaded.append((name, dest))
                        print("    -> %s (%d bytes)" % (name, got), file=sys.stderr)
                    update_profile_cache(cache, pkg, candidate, downloaded, use_cache)
                    return out_dir
                except (PlayUnavailable, EmptyDelivery) as e:
                    errors.append("%s: %s" % (label, e))
                    print("[!] profile failed: %s: %s" % (label, e), file=sys.stderr)
            raise RuntimeError("no working profile; tried %s" % "; ".join(errors))
        except Exception as e:  # dispenser rate-limit / transient / account-specific restrictions
            last = e
            print("[!] attempt %d failed: %s" % (attempt, e), file=sys.stderr)
            time.sleep(1.5)
    raise SystemExit("all %d attempts failed: %s" % (retries, last))


def main():
    ap = argparse.ArgumentParser(description="Download a Google Play app (base + splits) locally.")
    ap.add_argument("package", help="package name, e.g. com.anthropic.claude")
    ap.add_argument("--version", "-v", type=int, default=0, help="versionCode (default: latest)")
    ap.add_argument("--out", "-o", default=None, help="output dir (default: play_out/<package>)")
    ap.add_argument("--profile", default="auto", choices=["auto"] + sorted(PROFILES),
                    help="device profile to use (default: auto)")
    ap.add_argument("--extra-feature", action="append", default=[],
                    help="temporary Android feature to advertise; repeatable")
    ap.add_argument("--no-cache", action="store_true",
                    help="disable the per-package profile cache")
    a = ap.parse_args()
    out = a.out or os.path.join(HERE, "play_out", a.package)
    print("DONE: %s" % fetch(a.package, a.version, out,
                             profile_name=a.profile,
                             extra_features=a.extra_feature,
                             use_cache=not a.no_cache))


if __name__ == "__main__":
    main()
