"""Private FastLedger OCR fallback service; uploaded images are never persisted."""
from __future__ import annotations

import io
import logging
import os
from typing import Any

from fastapi import FastAPI, File, HTTPException, UploadFile
from PIL import Image, UnidentifiedImageError
from rapidocr_onnxruntime import RapidOCR

logging.basicConfig(level=os.environ.get("LOG_LEVEL", "INFO"))
logger = logging.getLogger("fastledger.ocr")
app = FastAPI(title="FastLedger OCR", docs_url=None, redoc_url=None)
_engine: RapidOCR | None = None
MAX_IMAGE_BYTES = int(os.environ.get("EBK_OCR_MAX_IMAGE_BYTES", "10485760"))


def engine() -> RapidOCR:
    """Lazily load the CPU OCR model once per process."""
    global _engine
    if _engine is None:
        _engine = RapidOCR()
    return _engine


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "up"}


@app.post("/ocr")
async def recognize(image: UploadFile = File(...)) -> dict[str, Any]:
    """Recognize an image held only in memory and return normalized text lines."""
    if image.content_type not in {"image/jpeg", "image/png", "image/webp", "image/bmp", "image/tiff"}:
        raise HTTPException(status_code=415, detail="unsupported image type")
    data = await image.read(MAX_IMAGE_BYTES + 1)
    if not data or len(data) > MAX_IMAGE_BYTES:
        raise HTTPException(status_code=413, detail="image too large or empty")
    try:
        with Image.open(io.BytesIO(data)) as source:
            source.verify()
        with Image.open(io.BytesIO(data)) as source:
            rgb = source.convert("RGB")
            result, _ = engine()(rgb)
    except (UnidentifiedImageError, OSError, ValueError) as exc:
        raise HTTPException(status_code=400, detail="invalid image") from exc
    except Exception as exc:  # Model/runtime failure must not leak implementation details.
        logger.warning("OCR inference unavailable (%s)", type(exc).__name__)
        raise HTTPException(status_code=503, detail="OCR unavailable") from exc
    lines: list[dict[str, Any]] = []
    for item in result or []:
        if not item or len(item) < 3:
            continue
        text, confidence = str(item[1]).strip(), float(item[2])
        if text and 0.0 <= confidence <= 1.0:
            lines.append({"text": text, "confidence": confidence})
    return {"engine": "rapidocr_onnxruntime", "lines": lines}
