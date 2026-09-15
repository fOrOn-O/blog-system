import json
from typing import Annotated

from langchain_core.tools import tool
from langgraph.prebuilt import ToolRuntime
from langgraph.prebuilt.tool_node import ToolInvocationError
from pydantic import Field

from app.agent.context import AgentContext
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
    ArticleStatus,
    CreateDraftInput,
    PositiveInt,
    UpdateDraftInput,
)

ARTICLE_FIELDS = {
    "id", "title", "content", "summary", "cover_image", "status",
    "version", "published_version", "tags",
}


def article_data(article: Article) -> dict:
    # Do not send author account details, counters, headers or client objects to the LLM.
    return article.model_dump(mode="json", include=ARTICLE_FIELDS)


def safe_tool_error(error: Exception) -> str:
    """Return fixed messages, never provider/backend exception text or validation inputs."""
    mappings = (
        (AuthenticationError, "authentication_required", "用户身份无效，请重新登录。"),
        (AuthorizationError, "permission_denied", "无权访问或修改该文章。"),
        (ArticleNotFoundError, "article_not_found", "文章不存在。"),
        (VersionConflictError, "version_conflict", "文章版本或状态已经变化，本次修改未执行。请告知用户，不要自动重试。"),
        (BlogBackendUnavailableError, "backend_unavailable", "博客服务暂不可用。写入是否完成尚不确定，不要自动重试。"),
        (BlogBackendError, "backend_error", "博客服务返回异常，无法确认本次操作结果。"),
        (BlogRequestError, "invalid_request", "博客服务拒绝了请求，请检查业务参数。"),
        ((ToolInvocationError, ValueError, TypeError), "invalid_arguments", "工具参数无效，请检查必填字段和版本号。"),
    )
    for error_type, code, message in mappings:
        if isinstance(error, error_type):
            return json.dumps({"ok": False, "error": code, "message": message}, ensure_ascii=False)
    return json.dumps({
        "ok": False, "error": "tool_error", "message": "工具执行失败，无法确认操作结果。"
    }, ensure_ascii=False)


@tool
async def get_article(article_id: PositiveInt, runtime: ToolRuntime[AgentContext]) -> dict:
    """读取当前用户自己的文章工作内容和版本，修改前应先读取。"""
    article = await runtime.context.blog_client.get_article(
        article_id, access_token=runtime.context.access_token
    )
    return {"ok": True, "data": article_data(article)}


@tool
async def list_my_articles(
    runtime: ToolRuntime[AgentContext],
    page: PositiveInt = 1,
    limit: Annotated[int, Field(strict=True, ge=1, le=100)] = 10,
    status: ArticleStatus | None = None,
) -> dict:
    """分页列出当前用户自己的文章，支持 draft、published、archived 状态筛选。"""
    result = await runtime.context.blog_client.list_my_articles(
        access_token=runtime.context.access_token, page=page, limit=limit, status=status
    )
    return {"ok": True, "data": {
        "articles": [article_data(article) for article in result.data],
        "meta": result.meta.model_dump(mode="json"),
    }}


@tool
async def create_draft(
    title: Annotated[str, Field(min_length=1, max_length=200)],
    content: Annotated[str, Field(min_length=1)],
    runtime: ToolRuntime[AgentContext],
    summary: str = "",
    cover_image: str = "",
    tag_ids: list[PositiveInt] | None = None,
) -> dict:
    """创建属于当前用户的文章草稿。content 使用 HTML；此操作不会发布文章。"""
    draft = CreateDraftInput(
        title=title, content=content, summary=summary, cover_image=cover_image,
        tag_ids=tag_ids if tag_ids is not None else [],
    )
    article = await runtime.context.blog_client.create_draft(
        draft, access_token=runtime.context.access_token
    )
    return {"ok": True, "data": article_data(article)}


@tool
async def update_draft(
    article_id: PositiveInt,
    expected_version: PositiveInt,
    runtime: ToolRuntime[AgentContext],
    title: Annotated[str | None, Field(min_length=1, max_length=200)] = None,
    content: Annotated[str | None, Field(min_length=1)] = None,
    summary: str | None = None,
    cover_image: str | None = None,
    tag_ids: list[PositiveInt] | None = None,
) -> dict:
    """基于读取到的 expected_version 修改 HTML 工作内容，不发布；冲突后不自动覆盖。"""
    draft = UpdateDraftInput(
        expected_version=expected_version, title=title, content=content,
        summary=summary, cover_image=cover_image, tag_ids=tag_ids,
    )
    article = await runtime.context.blog_client.update_draft(
        article_id, draft, access_token=runtime.context.access_token
    )
    return {"ok": True, "data": article_data(article)}


# Capability restriction applies to both model binding and actual ToolNode execution.
AGENT_TOOLS = (get_article, list_my_articles, create_draft, update_draft)
