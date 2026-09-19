from datetime import datetime
from typing import Annotated, Literal

from pydantic import BaseModel, ConfigDict, Field

ArticleStatus = Literal["draft", "published", "archived"]
PositiveInt = Annotated[int, Field(strict=True, gt=0)]


class CreateDraftInput(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    title: str = Field(min_length=1, max_length=200)
    content: str = Field(min_length=1)
    summary: str = ""
    cover_image: str = ""
    tag_ids: list[PositiveInt] = Field(default_factory=list)


class UpdateDraftInput(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    expected_version: PositiveInt
    title: str | None = Field(default=None, min_length=1, max_length=200)
    content: str | None = Field(default=None, min_length=1)
    summary: str | None = None
    cover_image: str | None = None
    tag_ids: list[PositiveInt] | None = None


class Tag(BaseModel):
    id: int
    name: str


class ArticleAuthor(BaseModel):
    id: int
    username: str
    email: str
    avatar: str
    bio: str
    role: str
    is_active: bool
    created_at: datetime


class Article(BaseModel):
    id: int
    status: ArticleStatus
    version: int
    published_version: int
    title: str
    content: str
    summary: str
    cover_image: str
    tags: list[Tag] = Field(default_factory=list)
    user: ArticleAuthor | None = None
    view_count: int
    like_count: int
    comment_count: int
    created_at: datetime
    updated_at: datetime


class Pagination(BaseModel):
    page: int
    limit: int
    total: int
    pages: int


class ArticlePage(BaseModel):
    data: list[Article]
    meta: Pagination


class TextFieldChange(BaseModel):
    changed: bool
    before: str
    after: str


class ArticleFieldChanges(BaseModel):
    title: TextFieldChange
    summary: TextFieldChange
    cover_image: TextFieldChange


class ContentBlock(BaseModel):
    type: Literal["heading", "paragraph", "list_item", "blockquote", "code_block"]
    text: str


class ContentBlockChange(BaseModel):
    operation: Literal["insert", "delete", "modify"]
    before_index: Annotated[int, Field(ge=0)] | None
    after_index: Annotated[int, Field(ge=0)] | None
    before: ContentBlock | None
    after: ContentBlock | None


class ContentChanges(BaseModel):
    changed: bool
    changes: list[ContentBlockChange]


class ArticleVersionDiff(BaseModel):
    article_id: PositiveInt
    from_version: PositiveInt
    to_version: PositiveInt
    field_changes: ArticleFieldChanges
    content: ContentChanges


class Heading(BaseModel):
    level: Annotated[int, Field(strict=True, ge=1, le=6)]
    text: str


class ChunkBlock(BaseModel):
    type: Literal["paragraph", "list_item", "blockquote", "code_block"]
    text: str


class ArticleChunk(BaseModel):
    index: Annotated[int, Field(strict=True, ge=0)]
    heading_path: list[Heading]
    blocks: list[ChunkBlock]
    text: str


class ArticleVersionChunks(BaseModel):
    user_id: PositiveInt
    article_id: PositiveInt
    version_no: PositiveInt
    chunks: list[ArticleChunk]


class ArticleVersion(BaseModel):
    model_config = ConfigDict(frozen=True)

    article_id: PositiveInt
    version_no: PositiveInt
    title: str
    content: str
    summary: str
    cover_image: str
