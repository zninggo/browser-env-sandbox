#!/usr/bin/env python3
"""Parse apify fingerprint-suite network.json → extract all real data →
generate JSON data file for bes runtime loading.

Usage: python3 parse_apify_data.py [network.json_path] [output_json_path]
Default: /tmp/.../network.json → data/fp_real_data.json
"""
import json, sys, os

NETWORK_FILE = sys.argv[1] if len(sys.argv) > 1 else "/tmp/fp_pkg/package/data_files/network_json/network.json"
OUTPUT_FILE = sys.argv[2] if len(sys.argv) > 2 else "data/fp_real_data.json"

def parse_stringified(val):
    if isinstance(val, str) and val.startswith("*STRINGIFIED*"):
        try:
            return json.loads(val[len("*STRINGIFIED*"):])
        except:
            return val
    return val

def safe_int(d, key, default=0):
    v = d.get(key, default)
    try: return int(v)
    except: return default

def safe_float(d, key, default=0.0):
    v = d.get(key, default)
    try: return float(v)
    except: return default

def main():
    with open(NETWORK_FILE) as f:
        network = json.load(f)

    nodes = {}
    for node in network.get("nodes", []):
        name = node.get("name", "")
        pv = node.get("possibleValues", [])
        unwrapped = [parse_stringified(v) for v in pv]
        nodes[name] = {
            "possibleValues": unwrapped,
            "parentNames": node.get("parentNames", []),
        }

    # --- VideoCards ---
    video_cards = [{"vendor": v["vendor"], "renderer": v["renderer"]}
                   for v in nodes.get("videoCard", {}).get("possibleValues", [])
                   if isinstance(v, dict) and "renderer" in v and "vendor" in v]

    # --- Screens ---
    screens = []
    for s in nodes.get("screen", {}).get("possibleValues", []):
        if isinstance(s, dict):
            screens.append({
                "width": safe_int(s, "width"),
                "height": safe_int(s, "height"),
                "avail_width": safe_int(s, "availWidth", safe_int(s, "width")),
                "avail_height": safe_int(s, "availHeight", safe_int(s, "height")),
                "color_depth": safe_int(s, "colorDepth", 24),
                "device_pixel_ratio": safe_float(s, "devicePixelRatio", 1.0),
                "inner_width": safe_int(s, "innerWidth", safe_int(s, "width")),
                "inner_height": safe_int(s, "innerHeight", safe_int(s, "availHeight", safe_int(s, "height"))),
                "outer_width": safe_int(s, "outerWidth", safe_int(s, "width")),
                "outer_height": safe_int(s, "outerHeight", safe_int(s, "height")),
            })

    # --- Fonts ---
    fonts_sets = []
    for fs in nodes.get("fonts", {}).get("possibleValues", []):
        if isinstance(fs, list):
            fonts_sets.append([f for f in fs if isinstance(f, str)])
        elif isinstance(fs, dict) and "fonts" in fs:
            fonts_sets.append([f for f in fs["fonts"] if isinstance(f, str)])

    # --- hardwareConcurrency ---
    hw_conc = [int(v) for v in nodes.get("hardwareConcurrency", {}).get("possibleValues", [])
               if str(v).strip().isdigit()]

    # --- deviceMemory ---
    dev_mem = []
    for v in nodes.get("deviceMemory", {}).get("possibleValues", []):
        try: dev_mem.append(int(v))
        except:
            try: dev_mem.append(int(float(v)))
            except: pass

    # --- Chrome UAs ---
    uas = [v for v in nodes.get("userAgent", {}).get("possibleValues", []) if isinstance(v, str)]
    chrome_uas = [ua for ua in uas if "Chrome/" in ua and "Edg/" not in ua and "OPR/" not in ua and "Brave" not in ua]

    output = {
        "version": "1.0",
        "source": "apify/fingerprint-suite",
        "source_url": "https://www.npmjs.com/package/fingerprint-generator",
        "generated_at": __import__("datetime").datetime.now().isoformat(),
        "stats": {
            "gpus": len(video_cards),
            "screens": len(screens),
            "fonts": len(fonts_sets),
            "chrome_uas": len(chrome_uas),
            "hardware_concurrency": len(hw_conc),
            "device_memory": len(dev_mem),
        },
        "gpus": video_cards,
        "screens": screens,
        "fonts": fonts_sets[:200],
        "chrome_uas": chrome_uas,
        "hardware_concurrency": hw_conc or [8, 12, 16, 4],
        "device_memory": [v for v in dev_mem if v >= 1] or [4, 8, 16, 32],
    }

    os.makedirs(os.path.dirname(OUTPUT_FILE) or ".", exist_ok=True)
    with open(OUTPUT_FILE, "w") as f:
        json.dump(output, f, ensure_ascii=False)
    print(f"Generated: {OUTPUT_FILE} ({os.path.getsize(OUTPUT_FILE)} bytes)")
    print(f"Stats: {output['stats']}")

if __name__ == "__main__":
    main()
