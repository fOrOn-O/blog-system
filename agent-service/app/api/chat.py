from functools import lru_cache
from typing import Literal

from fastapi import APIRouter, Depends, HTTPException
from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer
from pydantic import BaseModel, ConfigDict, Field

from app.agent.models import AgentResponse, AgentWorkspace
from app.agent.runner import AgentRunner
from app.agent.errors import AgentExecutionError
from app.clients.blog import BlogClient
from app.clients.errors import AuthenticationError, AuthorizationError, ArticleNotFoundError, BlogClientError
from app.core.config import get_settings
from app.core.model import ModelConfigurationError
from app.rag.errors import RagError
from app.core.rate_limit import limit_user

router = APIRouter(prefix="/api/v1/agent", tags=["agent"])
bearer = HTTPBearer(auto_error=False)


class ChatRequest(BaseModel):
    model_config = ConfigDict(extra="forbid", hide_input_in_errors=True)
    message: str = Field(min_length=1, max_length=12000)
    workspace: AgentWorkspace
    mode: Literal["question", "write"] = "question"


@lru_cache
def get_agent_runner() -> AgentRunner:
    return AgentRunner(settings=get_settings())


# 返回工厂，在 Go 校验身份与工作区后才创建模型；测试可替换工厂，无真实模型依赖。
def runner_factory():
    return get_agent_runner


@router.post("/chat", response_model=AgentResponse)
async def chat(request: ChatRequest, credentials: HTTPAuthorizationCredentials | None = Depends(bearer),
               factory=Depends(runner_factory)):
    if credentials is None or not credentials.credentials or not credentials.credentials.isascii() or any(c.isspace() for c in credentials.credentials):
        raise HTTPException(401, "请先登录")
    if not request.message.strip():
        raise HTTPException(422, "请输入消息")
    token = credentials.credentials
    try:
        async with BlogClient(get_settings()) as blog:
            user_id = await blog.get_authenticated_user_id(access_token=token)
            limit_user(user_id)
            # 工作区由应用选择，但仍须经 Go 校验。模型不能在工具参数中重新指定工作区。
            await blog.get_article_version(request.workspace.article_id, request.workspace.version_no, access_token=token)
            return await factory().run_with_response(request.message, access_token=token,
                                                    workspace=request.workspace, blog_client=blog, mode=request.mode)
    except AuthenticationError:
        raise HTTPException(401, "登录已过期") from None
    except AuthorizationError:
        raise HTTPException(403, "无权访问该文章") from None
    except ArticleNotFoundError:
        raise HTTPException(404, "文章或版本不存在") from None
    except (BlogClientError, AgentExecutionError, ModelConfigurationError, RagError):
        raise HTTPException(502, "助手暂时不可用，请稍后重试") from None
