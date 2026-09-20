"""浏览器 E2E 专用应用：只替换 LLM，使用真实 Router/Runner/ToolNode/BlogClient。"""
import json
import os

os.environ["LANGSMITH_TRACING"] = "false"
os.environ["LANGCHAIN_TRACING_V2"] = "false"

from fastapi import FastAPI, Header
from langchain_core.messages import AIMessage, HumanMessage
from app.api import chat
from app.api import knowledge
from app.agent.runner import AgentRunner
from app.core.config import Settings
from app.clients.blog import BlogClient
from app.rag.service import ArticleRagService
from app.rag.backend import LocalRetrievalBackend
from app.rag.store import QdrantChunkStore
from app.knowledge.service import PublishedKnowledgeService
from qdrant_client import AsyncQdrantClient
from test_rag import FakeEmbedding
from app.core import rate_limit


class WritingModel:
    def bind_tools(self, tools):
        return self

    async def ainvoke(self, messages):
        last = messages[-1]
        if isinstance(last, HumanMessage):
            if last.content.startswith('{'):
                data = json.loads(last.content)
                if "evidence" in data:
                    return AIMessage(content="根据检索到的文章：\n" + "\n".join(
                        f'{row["title"]} V{row["version_no"]}：{row["text"]}' for row in data["evidence"]))
            if "你好" in last.content:
                return AIMessage(content="你好，可以围绕当前版本提问或提出修改。")
            return AIMessage(content="", tool_calls=[{"name": "read_workspace_article", "args": {}, "id": "read"}])
        if last.name == "read_workspace_article":
            content = json.loads(last.content)["data"]["content"]
            return AIMessage(content="", tool_calls=[{"name": "submit_article_edit_proposal", "args": {
                "proposed_content": content + "<p>助手补充的结论。</p>",
                "change_summary": ["保留原文，补充结论段"]}, "id": "proposal"}])
        return AIMessage(content="已生成编辑提案，尚未保存。")


class E2EEmbedding(FakeEmbedding):
    def embed_documents(self, texts):
        return [[1. if any(word in text for word in ("RDB", "AOF", "快照策略")) else 0., 1.] + [0.] * 382 for text in texts]


settings = Settings(_env_file=None, groq_api_key="", blog_backend_url="http://127.0.0.1:18080")
test_limiter = rate_limit.RequestLimiter(200, 60)
rate_limit.get_request_limiter = lambda: test_limiter
chat.get_settings = lambda: settings
embedding = E2EEmbedding(settings)
exact = ArticleRagService(LocalRetrievalBackend(embedding, QdrantChunkStore(AsyncQdrantClient(":memory:"), settings)), top_k=5)
published_settings = settings.model_copy(update={"qdrant_collection": settings.published_knowledge_collection})
published = PublishedKnowledgeService(QdrantChunkStore(AsyncQdrantClient(":memory:"), published_settings, index_fields=("article_id", "version_no")), embedding=embedding)
knowledge.get_settings = lambda: settings
knowledge.get_published_service = lambda: published
knowledge.create_model = lambda _: WritingModel()
runner = AgentRunner(WritingModel(), settings=settings, rag_service=exact)
app = FastAPI()
app.include_router(chat.router)
app.include_router(knowledge.router)
app.dependency_overrides[chat.runner_factory] = lambda: lambda: runner


@app.post("/test/index/{article_id}/{version}")
async def index(article_id: int, version: int, authorization: str = Header()):
    # 仅浏览器测试应用注册，不进入生产路由；真实 BlogClient 校验 JWT 和版本权限。
    async with BlogClient(settings) as blog:
        count = await exact.index_article_version(article_id, version, access_token=authorization.removeprefix("Bearer "), blog_client=blog)
    return {"count": count}


@app.post("/test/rate-limit/{requests}")
def set_test_limit(requests: int):
    # 仅测试服务注册；各 E2E 可隔离额度，不增加生产重置入口。
    global test_limiter
    test_limiter = rate_limit.RequestLimiter(requests, 60)
    return {"ok": True}


@app.get("/health")
def health():
    return {"status": "ok"}
