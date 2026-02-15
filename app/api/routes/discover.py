#updates the route to be async and handle specific errors from the discover provider

from fastapi import APIRouter, HTTPException, Request
from app.schemas.discover import DiscoverSearchResponse
from app.services.discover_provider import discover_search, DiscoverNotConfiguredError
import httpx
from app.middleware.rate_limiter import rate_limit_user

router = APIRouter()

@rate_limit_user()
@router.get("/search", response_model=DiscoverSearchResponse)
async def search(request: Request, q: str):
    try:
        results = await discover_search(q)
        return DiscoverSearchResponse(results=results)

    except DiscoverNotConfiguredError as e:
        raise HTTPException(status_code=501, detail=str(e))

    except httpx.HTTPStatusError as e:
        # Tavily returned a non-2xx status (401/403/429/etc)
        status = e.response.status_code
        body = e.response.text[:500]  # avoid giant payloads
        raise HTTPException(status_code=502, detail=f"Tavily HTTP {status}: {body}")

    except httpx.RequestError as e:
        # network issue, DNS, timeout, TLS, etc.
        raise HTTPException(status_code=502, detail=f"Network error calling Tavily: {str(e)}")

    except Exception as e:
        raise HTTPException(status_code=502, detail=f"Discovery provider error: {str(e)}")
