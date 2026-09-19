from typing import Annotated

from pydantic import BaseModel, Field

from app.clients.models import Heading, PositiveInt


class RetrievedChunk(BaseModel):
    article_id: PositiveInt
    version_no: PositiveInt
    chunk_index: Annotated[int, Field(strict=True, ge=0)]
    heading_path: list[Heading]
    text: str
    score: float = Field(allow_inf_nan=False)


def assemble_context(chunks: list[RetrievedChunk]) -> str:
    """保留检索顺序和 Go 渲染出的标题上下文，不再次总结或重排。"""
    return "\n\n".join(
        f"[Article {c.article_id} / Version {c.version_no} / Chunk {c.chunk_index}]\n{c.text}"
        for c in chunks
    )
