from fastapi import APIRouter

from app.core.config import get_settings

router = APIRouter()


@router.get("/health")
def health() -> dict[str, str]:
    """Report process health without contacting external services."""
    return {"status": "ok", "service": get_settings().app_name}
