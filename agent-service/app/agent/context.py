from dataclasses import dataclass, field

from app.clients.blog import BlogClient
from app.clients.models import ArticleVersion
from app.agent.models import AgentWorkspace, ArticleEditProposal
from app.rag.service import ArticleRagService


@dataclass
class ProposalCapture:
    """只属于一次运行的临时结果，不保存 JWT，也不提供持久化或审批状态。"""

    canonical_version: ArticleVersion | None = field(default=None, repr=False)
    proposal: ArticleEditProposal | None = field(default=None, repr=False)


@dataclass(frozen=True)
class AgentContext:
    access_token: str = field(repr=False)
    blog_client: BlogClient = field(repr=False)
    rag_service: ArticleRagService | None = field(default=None, repr=False)
    workspace: AgentWorkspace | None = None
    proposal_capture: ProposalCapture | None = field(default=None, repr=False)
