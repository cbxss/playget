#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.9"
# dependencies = ["requests>=2.31", "protobuf>=5"]
# ///
"""
playget — download Google Play apps (incl. Play-only ones like Claude) locally.

Pure Python: anonymous Aurora-dispenser credentials + Google Play's protocol
(checkin -> uploadDeviceConfig -> details -> purchase -> delivery -> download),
reimplemented from Aurora's gplayapi. No JVM, no emulator, no token, no cloud.
Must run from a residential IP (Google blocks Play login from datacenter IPs).

Usage:
    uv run playget.py <package> [--version <versionCode>] [--out <dir>]

Examples:
    uv run playget.py com.anthropic.claude
    uv run playget.py com.anthropic.claude --version 26020937
"""
import argparse
import os
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


# ---------------------------------------------------------------- device profile
def load_device(path):
    d = {}
    with open(path, encoding="utf-8") as fh:
        for line in fh:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                d[k.strip().lower()] = v.strip()
    return d


DEV = load_device(os.path.join(HERE, "device.properties"))
def dv(k):
    return DEV.get(k.lower(), "")


def user_agent():
    return ("Android-Finsky/{vs} (api=3,versionCode={vc},sdk={sdk},device={dev},hardware={hw},"
            "product={pr},platformVersionRelease={pv},model={md},buildId={bid},isWideScreen=0,"
            "supportedAbis={abis})").format(
        vs=dv("vending.versionstring"), vc=dv("vending.version"), sdk=dv("build.version.sdk_int"),
        dev=dv("build.device"), hw=dv("build.hardware"), pr=dv("build.product"),
        pv=dv("build.version.release"), md=dv("build.model"), bid=dv("build.id"),
        abis=dv("platforms").replace(",", ";"))


# ---------------------------------------------------------------- protobuf builders
def android_build():
    b = gp.AndroidBuildProto()
    b.id = dv("build.fingerprint"); b.product = dv("build.hardware"); b.carrier = dv("build.brand")
    b.radio = dv("build.radio"); b.bootloader = dv("build.bootloader"); b.device = dv("build.device")
    b.sdkVersion = int(dv("build.version.sdk_int")); b.model = dv("build.model")
    b.manufacturer = dv("build.manufacturer"); b.buildProduct = dv("build.product")
    b.client = dv("client"); b.otaInstalled = False; b.timestamp = int(time.time())
    b.googleServices = int(dv("gsf.version"))
    return b


def device_config():
    c = gp.DeviceConfigurationProto()
    c.touchScreen = int(dv("touchscreen")); c.keyboard = int(dv("keyboard"))
    c.navigation = int(dv("navigation")); c.screenLayout = int(dv("screenlayout"))
    c.hasHardKeyboard = dv("hashardkeyboard") == "true"
    c.hasFiveWayNavigation = dv("hasfivewaynavigation") == "true"
    c.screenDensity = int(dv("screen.density")); c.screenWidth = int(dv("screen.width"))
    c.screenHeight = int(dv("screen.height")); c.glEsVersion = int(dv("gl.version"))
    for x in dv("platforms").split(","): c.nativePlatform.append(x)
    for x in dv("sharedlibraries").split(","): c.systemSharedLibrary.append(x)
    for x in dv("features").split(","): c.systemAvailableFeature.append(x)
    for x in dv("locales").split(","): c.systemSupportedLocale.append(x)
    for x in dv("gl.extensions").split(","): c.glExtension.append(x)
    return c


def checkin_request():
    r = gp.AndroidCheckinRequest()
    r.id = 0
    r.checkin.build.CopyFrom(android_build())
    r.checkin.lastCheckinMsec = 0
    r.checkin.cellOperator = dv("celloperator")
    r.checkin.simOperator = dv("simoperator")
    r.checkin.roaming = dv("roaming")
    r.checkin.userNumber = 0
    r.locale = LOCALE
    r.timeZone = dv("timezone") or "America/Los_Angeles"
    r.version = 3
    r.deviceConfiguration.CopyFrom(device_config())
    r.fragment = 0
    return r


# ---------------------------------------------------------------- HTTP layer
def fdfe_headers(token, gsfid, checkin_token="", config_token=""):
    h = {
        "Authorization": "Bearer " + token,
        "User-Agent": user_agent(),
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
        "X-DFE-MCCMNC": dv("simoperator"),
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


def checkin(sess):
    req = checkin_request()
    h = {"app": "com.google.android.gms",
         "User-Agent": "GoogleAuth/1.4 (%s %s)" % (dv("build.device"), dv("build.id")),
         "Content-Type": "application/x-protobuffer",
         "Host": "android.clients.google.com"}
    r = sess.post(BASE + "/checkin", data=req.SerializeToString(), headers=h, timeout=40)
    r.raise_for_status()
    resp = gp.AndroidCheckinResponse()
    resp.ParseFromString(r.content)
    return "%x" % resp.androidId, resp.deviceCheckinConsistencyToken


def upload_device_config(sess, token, gsfid, ckt):
    up = gp.UploadDeviceConfigRequest()
    up.deviceConfiguration.CopyFrom(device_config())
    r = sess.post(BASE + "/fdfe/uploadDeviceConfig", data=up.SerializeToString(),
                  headers=fdfe_headers(token, gsfid, ckt), timeout=40)
    r.raise_for_status()
    return wrapper(r.content).payload.uploadDeviceConfigResponse.uploadDeviceConfigToken


def app_details(sess, token, gsfid, ckt, cft, pkg):
    r = sess.get(BASE + "/fdfe/details", params={"doc": pkg},
                 headers=fdfe_headers(token, gsfid, ckt, cft), timeout=40)
    r.raise_for_status()
    doc = wrapper(r.content).payload.detailsResponse.docV2
    return doc.details.appDetails.versionCode, (doc.title or pkg)


def delivery_token(sess, token, gsfid, ckt, cft, pkg, vc, ot=1):
    r = sess.post(BASE + "/fdfe/purchase", params={"ot": str(ot), "doc": pkg, "vc": str(vc)},
                  data="", headers=fdfe_headers(token, gsfid, ckt, cft), timeout=40)
    r.raise_for_status()
    return wrapper(r.content).payload.buyResponse.downloadToken


def delivery(sess, token, gsfid, ckt, cft, pkg, vc, dtok, ot=1):
    params = {"ot": str(ot), "doc": pkg, "vc": str(vc)}
    if dtok:
        params["dtok"] = dtok
    r = sess.get(BASE + "/fdfe/delivery", params=params,
                 headers=fdfe_headers(token, gsfid, ckt, cft), timeout=40)
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


def fetch(pkg, version, out_dir, retries=4):
    last = None
    for attempt in range(1, retries + 1):
        try:
            email, token = dispenser()
            print("[*] dispenser: %s" % email, file=sys.stderr)
            sess = requests.Session()
            gsfid, ckt = checkin(sess)
            cft = upload_device_config(sess, token, gsfid, ckt)
            print("[*] auth ok. gsfId=%s" % gsfid, file=sys.stderr)
            if version:
                vc, title = version, pkg
            else:
                vc, title = app_details(sess, token, gsfid, ckt, cft, pkg)
            print("[*] %s  versionCode=%s" % (title, vc), file=sys.stderr)
            dtok = delivery_token(sess, token, gsfid, ckt, cft, pkg, vc)
            files, cookies = delivery(sess, token, gsfid, ckt, cft, pkg, vc, dtok)
            if not files:
                raise RuntimeError("delivery returned no files (not purchasable / wrong version?)")
            print("[*] files: %d" % len(files), file=sys.stderr)
            os.makedirs(out_dir, exist_ok=True)
            for name, url in files:
                got = download(url, cookies, os.path.join(out_dir, name))
                print("    -> %s (%d bytes)" % (name, got), file=sys.stderr)
            return out_dir
        except Exception as e:  # dispenser rate-limit / transient — retry with fresh creds
            last = e
            print("[!] attempt %d failed: %s" % (attempt, e), file=sys.stderr)
            time.sleep(1.5)
    raise SystemExit("all %d attempts failed: %s" % (retries, last))


def main():
    ap = argparse.ArgumentParser(description="Download a Google Play app (base + splits) locally.")
    ap.add_argument("package", help="package name, e.g. com.anthropic.claude")
    ap.add_argument("--version", "-v", type=int, default=0, help="versionCode (default: latest)")
    ap.add_argument("--out", "-o", default=None, help="output dir (default: play_out/<package>)")
    a = ap.parse_args()
    out = a.out or os.path.join(HERE, "play_out", a.package)
    print("DONE: %s" % fetch(a.package, a.version, out))


if __name__ == "__main__":
    main()
