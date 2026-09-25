"""请求内的脱敏诊断；不参与工具选择、提案校验或错误映射。"""

from contextlib import contextmanager
from contextvars import ContextVar
from dataclasses import dataclass
import json
import logging
from uuid import uuid4

logger = logging.getLogger(__name__)
request_id: ContextVar[str | None] = ContextVar("agent_request_id", default=None)


@dataclass
class RunObservation:
    model_rounds: int = 0


@dataclass(frozen=True)
class ToolObservation:
    name: str
    article_id: int | None
    version_no: int | None


_run: ContextVar[RunObservation | None] = ContextVar("agent_run_observation", default=None)
_tool: ContextVar[ToolObservation | None] = ContextVar("agent_tool_observation", default=None)


def record(stage: str, **fields) -> None:
    # 调用方只传固定分类、计数和可信工作区 ID，禁止传入消息或异常对象。
    logger.info("agent_observation %s", json.dumps(
        {"request_id": request_id.get(), "stage": stage, **fields}, ensure_ascii=True,
    ))


@contextmanager
def run_observation():
    identity = request_id.set(request_id.get() or uuid4().hex)
    observation = RunObservation()
    token = _run.set(observation)
    try:
        yield observation
    finally:
        _run.reset(token)
        request_id.reset(identity)


def model_returned(reply, tool_names: set[str]) -> None:
    observation = _run.get()
    if observation is not None:
        observation.model_rounds += 1
    finish_reason = reply.response_metadata.get("finish_reason")
    if finish_reason not in (None, "stop", "tool_calls", "length", "function_call", "content_filter"):
        finish_reason = "other"
    # 只计算文本长度，不序列化正文、推理内容或工具参数。
    content = reply.text
    record("model", model_round=observation.model_rounds if observation else None,
           finish_reason=finish_reason, tool_call_count=len(reply.tool_calls),
           tool_names=[call["name"] if call["name"] in tool_names else "unregistered_tool"
                       for call in reply.tool_calls],
           invalid_tool_call_count=len(reply.invalid_tool_calls),
           content_empty=not bool(content), content_length=len(content))


async def observe_tool_call(request, execute):
    workspace = request.runtime.context.workspace
    observation = ToolObservation(
        request.tool.name if request.tool is not None else "unregistered_tool",
        workspace.article_id if workspace else None,
        workspace.version_no if workspace else None,
    )
    token = _tool.set(observation)
    try:
        result = await execute(request)
        # 未注册工具由 ToolNode 直接生成错误消息，不经过 safe_tool_error。
        if request.tool is None:
            tool_error("unregistered_tool")
        return result
    finally:
        _tool.reset(token)


def tool_error(reason: str) -> None:
    observation = _tool.get()
    record("tool", tool_name=observation.name if observation else "unknown_tool",
           result="error", reason=reason)


def snapshot_validation(response_validation: bool | None, identity_validation: bool | None) -> None:
    observation = _tool.get()
    if observation is not None and observation.name == "read_workspace_article":
        record("read_workspace_article", article_id=observation.article_id,
               version_no=observation.version_no, response_validation=response_validation,
               identity_validation=identity_validation)


def proposal_stage(stage: str) -> None:
    observation = _tool.get()
    record("submit_article_edit_proposal", proposal_stage=stage,
           article_id=observation.article_id if observation else None,
           version_no=observation.version_no if observation else None)
