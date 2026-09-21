from fastapi import APIRouter, Depends, HTTPException
from fastapi.security import HTTPAuthorizationCredentials
from pydantic import BaseModel, ConfigDict, Field

from app.api.chat import bearer
from app.clients.blog import BlogClient
from app.clients.errors import AuthenticationError, AuthorizationError, BlogClientError
from app.core.config import get_settings
from app.core.model import create_model, ModelConfigurationError
from app.core.rag import get_published_service
from app.agent.errors import AgentExecutionError
from app.knowledge.answer import grounded_answer
from app.knowledge.models import GroundedAnswer
from app.rag.errors import RagError
from app.core.rate_limit import limit_user

router = APIRouter(prefix="/api/v1/agent/knowledge", tags=["knowledge"])


class KnowledgeQuestion(BaseModel):
    model_config = ConfigDict(extra="forbid", hide_input_in_errors=True)
    query: str = Field(min_length=1, max_length=12000)


async def authenticated_blog(credentials: HTTPAuthorizationCredentials | None = Depends(bearer)):
    if credentials is None or not credentials.credentials.isascii() or not credentials.credentials or any(c.isspace() for c in credentials.credentials):
        raise HTTPException(401, "请先登录")
    try:
        async with BlogClient(get_settings()) as blog:
            token = credentials.credentials
            # 认证在 API 边界完成；用户 ID 仅作限流 key，不传入站点检索。
            user_id = await blog.get_authenticated_user_id(access_token=token)
            limit_user(user_id)
            yield blog, token
    except AuthenticationError:
        raise HTTPException(401, "登录已过期") from None
    except AuthorizationError:
        raise HTTPException(403, "无权访问") from None
    except AgentExecutionError as error:
        raise HTTPException(error.status_code, {"code": error.code, "message": "助手暂不可用，请稍后重试。"}) from None
    except (BlogClientError, RagError, ModelConfigurationError):
        raise HTTPException(502, "知识服务暂不可用；同步失败可显式重试，业务发布状态不受影响") from None


@router.post("/chat", response_model=GroundedAnswer)
async def knowledge_chat(request: KnowledgeQuestion, auth=Depends(authenticated_blog)):
    if not request.query.strip():
        raise HTTPException(422, "请输入问题")
    blog, token = auth
    hits = await get_published_service().search_published_knowledge(request.query, access_token=token, blog_client=blog)
    return await grounded_answer(request.query, hits, model_factory=lambda: create_model(get_settings()),
                                 scope="本站当前已发布文章中")


@router.post("/sync/{article_id}")
async def sync_article(article_id: int, auth=Depends(authenticated_blog)):
    if article_id <= 0:
        raise HTTPException(422, "文章 ID 必须为正整数")
    blog, token = auth
    count = await get_published_service().sync_article(article_id, access_token=token, blog_client=blog)
    return {"article_id": article_id, "indexed_chunks": count}
