#!/usr/bin/env python3
"""Download SWE-bench Lite from HuggingFace and write per-instance JSON files
with the gold patch included (needed for oracle context gathering).

Usage:
    pip install datasets
    python prepare_dataset.py

Outputs:
    ../dataset/test/<instance_id>.json   (one per instance)
    ../dataset.zip                       (zipped dataset ready for Docker)
"""

import json
import os
import zipfile
from pathlib import Path

from datasets import load_dataset

SCRIPT_DIR = Path(__file__).resolve().parent
DATASET_DIR = SCRIPT_DIR.parent / "dataset" / "test"
ZIP_PATH = SCRIPT_DIR.parent / "dataset.zip"


def main():
    print("Downloading princeton-nlp/SWE-bench_Lite ...")
    ds = load_dataset("princeton-nlp/SWE-bench_Lite", split="test")

    DATASET_DIR.mkdir(parents=True, exist_ok=True)

    for existing in DATASET_DIR.glob("*.json"):
        existing.unlink()

    count = 0
    for row in ds:
        instance = {
            "instance_id": row["instance_id"],
            "repo": row["repo"],
            "base_commit": row["base_commit"],
            "problem_statement": row["problem_statement"],
            "patch": row["patch"],
        }
        out_path = DATASET_DIR / f"{row['instance_id']}.json"
        out_path.write_text(json.dumps(instance, indent=2, ensure_ascii=False) + "\n")
        count += 1

    print(f"Wrote {count} instance files to {DATASET_DIR}")

    if ZIP_PATH.exists():
        ZIP_PATH.unlink()

    with zipfile.ZipFile(ZIP_PATH, "w", zipfile.ZIP_DEFLATED) as zf:
        for json_file in sorted(DATASET_DIR.glob("*.json")):
            arcname = f"test/{json_file.name}"
            zf.write(json_file, arcname)

    size_mb = ZIP_PATH.stat().st_size / (1024 * 1024)
    print(f"Wrote {ZIP_PATH} ({size_mb:.1f} MB)")


if __name__ == "__main__":
    main()
