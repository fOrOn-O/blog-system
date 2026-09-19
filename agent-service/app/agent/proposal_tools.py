from langchain_core.messages import AIMessage, ToolMessage
from langchain_core.tools import tool
from langgraph.prebuilt import ToolRuntime

from app.agent.context import AgentContext
from app.agent.errors import CanonicalReadRequiredError, MultipleProposalSubmissionsError, ProposalAlreadySubmittedError, WorkspaceRequiredError
from app.agent.models import ArticleEditProposal
from app.agent.tools import get_article, get_version_diff, list_my_articles, search_article_version


def _workspace_context(runtime: ToolRuntime[AgentContext]):
    context = runtime.context
    if context.workspace is None or context.proposal_capture is None:
        raise WorkspaceRequiredError()
    return context, context.workspace, context.proposal_capture


@tool
async def read_workspace_article(runtime: ToolRuntime[AgentContext]) -> dict:
    """读取活动工作区绑定的完整历史 HTML 快照；生成编辑提案前必须先调用并阅读结果。"""
    context, workspace, capture = _workspace_context(runtime)
    version = await context.blog_client.get_article_version(
        workspace.article_id, workspace.version_no, access_token=context.access_token,
    )
    capture.canonical_version = version
    return {"ok": True, "data": version.model_dump(mode="json")}


@tool
async def submit_article_edit_proposal(
    proposed_content: str, change_summary: list[str], runtime: ToolRuntime[AgentContext],
) -> dict:
    """提交完整 HTML 编辑提案及变更摘要，仅返回本次运行的建议，不保存、不创建版本、不发布。"""
    context, workspace, capture = _workspace_context(runtime)
    # 要求读取结果已进入之前的图状态，禁止在同一轮并行调用读取和提交来跳过阅读。
    messages = runtime.state.get("messages", [])
    # 同轮多次提交全部拒绝，不让网络完成顺序决定保留哪一份提案。
    latest_reply = next((message for message in reversed(messages) if isinstance(message, AIMessage)), None)
    if latest_reply is not None and sum(
        call["name"] == "submit_article_edit_proposal" for call in latest_reply.tool_calls
    ) > 1:
        raise MultipleProposalSubmissionsError()
    if capture.proposal is not None:
        raise ProposalAlreadySubmittedError()
    if capture.canonical_version is None or not any(
        isinstance(message, ToolMessage) and message.name == "read_workspace_article" and message.status == "success"
        for message in messages
    ):
        raise CanonicalReadRequiredError()
    await context.blog_client.get_article_version(
        workspace.article_id, workspace.version_no, access_token=context.access_token,
    )
    proposal = ArticleEditProposal(
        article_id=workspace.article_id, base_version_no=workspace.version_no,
        proposed_content=proposed_content, change_summary=change_summary,
    )
    # 接受单次提交后不允许覆盖；检查和赋值之间没有 await。
    if capture.proposal is not None:
        raise ProposalAlreadySubmittedError()
    capture.proposal = proposal
    return {"ok": True, "data": proposal.model_dump(mode="json")}


# 与旧入口明确分开；模型绑定与 ToolNode 均使用这份只读白名单。
PROPOSAL_TOOLS = (
    get_article, list_my_articles, get_version_diff, search_article_version,
    read_workspace_article, submit_article_edit_proposal,
)
