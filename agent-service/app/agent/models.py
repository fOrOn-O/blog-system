from typing import Annotated

from pydantic import BaseModel, ConfigDict, Field, field_validator

from app.agent.html_validation import validate_proposed_html
from app.clients.models import PositiveInt
from app.knowledge.models import KnowledgeSource


class AgentWorkspace(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    article_id: PositiveInt
    version_no: PositiveInt


class ArticleEditProposal(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    article_id: PositiveInt
    base_version_no: PositiveInt
    proposed_content: str
    change_summary: list[Annotated[str, Field(strict=True, min_length=1)]] = Field(min_length=1)

    @field_validator("proposed_content")
    @classmethod
    def validate_content(cls, value: str) -> str:
        return validate_proposed_html(value)

    @field_validator("change_summary")
    @classmethod
    def validate_summary(cls, value: list[str]) -> list[str]:
        if any(not item.strip() for item in value):
            raise ValueError("Change summaries must not be blank")
        return value


class AgentResponse(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    answer: str
    proposal: ArticleEditProposal | None = None
    has_evidence: bool | None = None
    sources: list[KnowledgeSource] = Field(default_factory=list)
