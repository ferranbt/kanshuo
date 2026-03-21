#!/usr/bin/env python3
"""
OCR Service - Performs Chinese text recognition using EasyOCR
Runs as HTTP server on port 5001
"""
import warnings
import os
import logging
import argparse

warnings.filterwarnings("ignore")
os.environ["PYTHONWARNINGS"] = "ignore"
logging.getLogger("easyocr").setLevel(logging.ERROR)

import easyocr
from flask import Flask, request, jsonify
from dataclasses import dataclass, asdict
import base64
from io import BytesIO
from PIL import Image

app = Flask(__name__)

parser = argparse.ArgumentParser(description="OCR Service")
parser.add_argument(
    "--traditional",
    action="store_true",
    default=False,
    help="Use traditional Chinese model (ch_tra) instead of simplified (ch_sim)",
)
args = parser.parse_args()

# Initialize OCR reader globally
lang = "ch_tra" if args.traditional else "ch_sim"
print(f"Loading EasyOCR model (language: {lang})...")
reader = easyocr.Reader([lang], gpu=False, verbose=False)
print("EasyOCR ready!")


@dataclass
class TextRegion:
    text: str
    x1: int
    y1: int
    x2: int
    y2: int
    confidence: float


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok", "service": "ocr"})


@app.route("/ocr", methods=["POST"])
def ocr():
    """
    Process image and return OCR results
    Input: JSON with image_path or base64 image data
    Output: JSON with text regions
    """
    try:
        data = request.get_json()

        # Get image from path or base64
        if "image_path" in data:
            image_path = data["image_path"]
            results = reader.readtext(image_path)
        elif "image_base64" in data:
            # Decode base64 image
            img_data = base64.b64decode(data["image_base64"])
            img = Image.open(BytesIO(img_data))
            results = reader.readtext(img)
        else:
            return jsonify({"error": "Missing image_path or image_base64"}), 400

        # Process results
        regions = []
        for box, text, confidence in results:
            # box is [[x1,y1], [x2,y1], [x2,y2], [x1,y2]]
            x1 = int(min(p[0] for p in box))
            y1 = int(min(p[1] for p in box))
            x2 = int(max(p[0] for p in box))
            y2 = int(max(p[1] for p in box))

            regions.append(
                TextRegion(
                    text=text,
                    x1=x1,
                    y1=y1,
                    x2=x2,
                    y2=y2,
                    confidence=float(confidence),
                )
            )

        return jsonify({"ok": True, "regions": [asdict(r) for r in regions]})

    except Exception as e:
        return jsonify({"ok": False, "error": str(e)}), 500


if __name__ == "__main__":
    print("Starting OCR service on http://localhost:5010")
    app.run(host="0.0.0.0", port=5010, threaded=True)
