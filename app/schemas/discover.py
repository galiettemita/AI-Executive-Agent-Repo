from pydantic import BaseModel
from typing import Optional

class DiscoverResult(BaseModel):
    title: str
    url: str
    snippet: Optional[str] = None
    source: str = "serpapi"
    retailer_domain: Optional[str] = None  # NEW

class DiscoverSearchResponse(BaseModel):
    results: list[DiscoverResult]
