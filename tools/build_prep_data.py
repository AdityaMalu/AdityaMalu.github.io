"""Builds prep/data.json for the /prep hub: quiz questions and topic lists from the three Concepts Steppers,
plus AlgoScope problem ids and MachineCodingLab project ids.

The steppers keep their content in JavaScript, so this loads each live page in a headless browser and reads
the topic list the page itself uses. Re-run after a stepper's content changes:

    python tools/build_prep_data.py            (needs: pip install playwright && playwright install chromium)
"""
import json, os, urllib.request
from playwright.sync_api import sync_playwright

SITE = "https://adityamalu.github.io"
OUT = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "prep", "data.json")

STEPPERS = {"java": f"{SITE}/java-visualizer/", "db": f"{SITE}/DBVisualizer/", "os": f"{SITE}/OSVisualizer/"}
READ_TOPICS = """() => T.map(t => ({ id: t.id, no: t.no, title: t.title, imp: t.imp,
  quiz: (t.quiz || []).map(([q, o, a, w, l]) => [q, o, a, w, l]) }))"""


def get_json(url):
    with urllib.request.urlopen(url, timeout=60) as r:
        return json.load(r)


def main():
    out = {"steppers": {}}
    with sync_playwright() as p:
        browser = p.chromium.launch()
        page = browser.new_page()
        for key, url in STEPPERS.items():
            page.goto(url, wait_until="load")
            topics = page.evaluate(READ_TOPICS)
            out["steppers"][key] = topics
            print(f"{key}: {len(topics)} topics, {sum(len(t['quiz']) for t in topics)} questions")
        browser.close()
    out["algo"] = [[x["id"], x["title"], x["topic"]] for x in get_json(f"{SITE}/algoscope/data/index.json")]
    print(f"algo: {len(out['algo'])} problems")
    cat = get_json("https://raw.githubusercontent.com/AdityaMalu/machine-coding-lab/main/projects/catalog.json")
    projects = cat["projects"] if isinstance(cat, dict) else cat
    out["lab"] = [[pr["id"], pr["title"]] for pr in projects]
    print(f"lab: {len(out['lab'])} projects")
    with open(OUT, "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, separators=(",", ":"))
    print(f"wrote {OUT} ({os.path.getsize(OUT) // 1024} KB)")


if __name__ == "__main__":
    main()
