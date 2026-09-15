from dataclasses import dataclass, field

from app.clients.blog import BlogClient


@dataclass(frozen=True)
class AgentContext:
    access_token: str = field(repr=False)
    blog_client: BlogClient = field(repr=False)
