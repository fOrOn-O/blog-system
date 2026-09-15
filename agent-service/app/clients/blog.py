from typing import Any, Self, TypeVar

import httpx
from pydantic import BaseModel

from app.clients.errors import (
    ArticleNotFoundError,
    AuthenticationError,
    AuthorizationError,
    BlogBackendError,
    BlogBackendUnavailableError,
    BlogRequestError,
    VersionConflictError,
)
from app.clients.models import (
    Article,
    ArticlePage,
    ArticleStatus,
    CreateDraftInput,
    UpdateDraftInput,
)
from app.core.config import Settings, get_settings

ResponseModel = TypeVar("ResponseModel", bound=BaseModel)


class _ArticleEnvelope(BaseModel):
    data: Article


class BlogClient:
    """Reuse connections, but pass user credentials separately on every call."""

    def __init__(
        self,
        settings: Settings | None = None,
        *,
        transport: httpx.AsyncBaseTransport | None = None,
    ) -> None:
        settings = settings if settings is not None else get_settings()
        self._http = httpx.AsyncClient(
            base_url=str(settings.blog_backend_url),
            timeout=settings.blog_backend_timeout_seconds,
            transport=transport,
            follow_redirects=False,
            trust_env=False,
        )

    async def __aenter__(self) -> Self:
        await self._http.__aenter__()
        return self

    async def __aexit__(self, exc_type, exc_value, traceback) -> None:
        await self.aclose()

    async def aclose(self) -> None:
        await self._http.aclose()

    async def get_article(self, article_id: int, *, access_token: str) -> Article:
        result = await self._request(
            "GET", self._article_path(article_id), access_token, _ArticleEnvelope
        )
        return result.data

    async def list_my_articles(
        self,
        *,
        access_token: str,
        page: int = 1,
        limit: int = 10,
        status: ArticleStatus | None = None,
    ) -> ArticlePage:
        self._require_positive_int(page, "page")
        self._require_positive_int(limit, "limit")
        if limit > 100:
            raise ValueError("limit must not exceed 100")
        params: dict[str, str | int] = {"page": page, "limit": limit}
        if status is not None:
            if status not in ("draft", "published", "archived"):
                raise ValueError("Invalid article status")
            params["status"] = status
        return await self._request(
            "GET", "/api/v1/agent/articles", access_token, ArticlePage, params=params
        )

    async def create_draft(self, draft: CreateDraftInput, *, access_token: str) -> Article:
        result = await self._request(
            "POST", "/api/v1/agent/articles", access_token, _ArticleEnvelope,
            json=draft.model_dump(mode="json"),
        )
        return result.data

    async def update_draft(
        self, article_id: int, draft: UpdateDraftInput, *, access_token: str
    ) -> Article:
        result = await self._request(
            "PUT", self._article_path(article_id) + "/draft", access_token, _ArticleEnvelope,
            json=draft.model_dump(mode="json", exclude_unset=True, exclude_none=True),
        )
        return result.data

    async def publish_article(
        self, article_id: int, *, expected_version: int, access_token: str
    ) -> Article:
        self._require_positive_int(expected_version, "expected_version")
        result = await self._request(
            "POST", self._article_path(article_id) + "/publish", access_token, _ArticleEnvelope,
            json={"expected_version": expected_version},
        )
        return result.data

    async def archive_article(self, article_id: int, *, access_token: str) -> Article:
        result = await self._request(
            "POST", self._article_path(article_id) + "/archive", access_token, _ArticleEnvelope,
        )
        return result.data

    @staticmethod
    def _require_positive_int(value: int, name: str) -> None:
        if type(value) is not int or value <= 0:
            raise ValueError(f"{name} must be a positive integer")

    @classmethod
    def _article_path(cls, article_id: int) -> str:
        cls._require_positive_int(article_id, "article_id")
        return f"/api/v1/agent/articles/{article_id}"

    async def _request(
        self,
        method: str,
        path: str,
        access_token: str,
        model: type[ResponseModel],
        *,
        params: dict[str, str | int] | None = None,
        json: dict[str, Any] | None = None,
    ) -> ResponseModel:
        if (
            not isinstance(access_token, str)
            or not access_token
            or not access_token.isascii()
            or any(c.isspace() for c in access_token)
        ):
            raise ValueError("access_token must be a nonempty token without whitespace")
        try:
            response = await self._http.request(
                method, path, params=params, json=json,
                headers={"Authorization": f"Bearer {access_token}"},
            )
        except httpx.RequestError:
            # Raw httpx errors may retain headers or echo a token; do not chain them.
            raise BlogBackendUnavailableError(
                "Go backend is unavailable or the request timed out", method=method, path=path
            ) from None

        status = response.status_code
        context = {"method": method, "path": path, "status_code": status}
        if not response.is_success:
            if 400 <= status < 500:
                message = "Go backend rejected the request"
                try:
                    body = response.json()
                    if isinstance(body, dict) and isinstance(body.get("message"), str):
                        message = body["message"].replace(access_token, "[REDACTED]")
                except ValueError:
                    pass
                error_type = {
                    401: AuthenticationError, 403: AuthorizationError,
                    404: ArticleNotFoundError, 409: VersionConflictError,
                }.get(status, BlogRequestError)
                raise error_type(message, **context)
            # Neither server error bodies nor redirects are safe business messages.
            raise BlogBackendError("Go backend returned an unexpected HTTP status", **context)

        try:
            body = response.json()
            if (
                not isinstance(body, dict)
                or type(body.get("code")) is not int
                or body["code"] != status
            ):
                raise ValueError("Invalid response envelope")
            return model.model_validate(body)
        except ValueError:
            raise BlogBackendError("Go backend returned an invalid response", **context) from None
