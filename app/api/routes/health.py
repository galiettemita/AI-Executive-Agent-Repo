import os
from fastapi import APIRouter

router = APIRouter()

@router.get("/health")
def health():
    serpapi_ok = bool(os.getenv("SERPAPI_API_KEY"))
    openai_ok = bool(os.getenv("OPENAI_API_KEY"))
    return {
        "status": "ok",
        "serpapi_configured": serpapi_ok,
        "openai_configured": openai_ok,
    }
