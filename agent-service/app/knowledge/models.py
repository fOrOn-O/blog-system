from pydantic import BaseModel, ConfigDict, Field

from app.clients.models import Heading, PositiveInt


class KnowledgeSource(BaseModel):
    model_config = ConfigDict(frozen=True)
    article_id: PositiveInt
    title: str
    version_no: PositiveInt
    chunk_index: int = Field(ge=0, strict=True)
    heading_path: list[Heading]


class KnowledgeHit(KnowledgeSource):
    text: str
    score: float = Field(allow_inf_nan=False)


class GroundedAnswer(BaseModel):
    answer: str
    has_evidence: bool
    sources: list[KnowledgeSource] = Field(default_factory=list)
