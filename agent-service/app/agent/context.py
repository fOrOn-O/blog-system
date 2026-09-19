from dataclasses import dataclass, field

from app.clients.blog import BlogClient
from app.rag.service import ArticleRagService


@dataclass(frozen=True)
class AgentContext:
    access_token: str = field(repr=False)
    blog_client: BlogClient = field(repr=False)
    rag_service: ArticleRagService | None = field(default=None, repr=False)
