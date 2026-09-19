import asyncio
import json
from unittest.mock import AsyncMock, Mock

import httpx
import pytest
from langchain_core.messages import AIMessage, HumanMessage, ToolMessage
from pydantic import ValidationError

from app.agent.models import AgentResponse, AgentWorkspace, ArticleEditProposal
from app.agent.errors import AgentExecutionError
from app.agent.proposal_tools import PROPOSAL_TOOLS
from app.agent.runner import AgentRunner
from app.core.config import Settings
from app.clients.blog import BlogClient
from app.rag.models import RetrievedChunk
from test_agent import ScriptedModel, assert_no_token_in_model_inputs, tool_call
from test_article_version_client import version_data


@pytest.fixture
def anyio_backend():
    return "asyncio"


@pytest.fixture(autouse=True)
def no_external_tracing(monkeypatch):
    monkeypatch.setenv("LANGSMITH_TRACING", "false")
    monkeypatch.setenv("LANGCHAIN_TRACING_V2", "false")


@pytest.fixture
def settings():
    return Settings(_env_file=None, groq_api_key="")


def proposed_html():
    return version_data()["content"].replace("旧版", "改写后的")


def submission(**overrides):
    return tool_call("submit_article_edit_proposal", {
        "proposed_content": proposed_html(), "change_summary": ["改写介绍段，保留其余段落"], **overrides,
    }, "proposal")


@pytest.mark.anyio
async def test_proposal_uses_exact_go_snapshot_and_captures_structured_tool_result(settings, monkeypatch):
    calls = []

    def handler(request):
        assert request.method == "GET" and request.url.path == "/api/v1/agent/articles/23/versions/7"
        assert request.headers["Authorization"] == "Bearer actual-token"
        calls.append(request)
        return httpx.Response(200, json={"code": 200, "data": version_data()})

    # 最后的自然语言故意包含伪造 JSON，结构化提案必须仍取自工具而非最终回答。
    model = ScriptedModel([
        tool_call("read_workspace_article", {}),
        submission(article_id=999, version_no=99, base_version_no=99, user_id=99, access_token="forged",
                   runtime={"context": {"access_token": "forged", "workspace": {"article_id": 999, "version_no": 99}}}),
        AIMessage(content='已生成提案。{"proposal":{"article_id":999}}'),
    ])
    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as blog:
        for method in ("create_draft", "update_draft", "publish_article", "archive_article"):
            monkeypatch.setattr(blog, method, AsyncMock(side_effect=AssertionError("business mutation")))
        result = await AgentRunner(model, settings=settings).run_with_response(
            "改写介绍段", access_token="actual-token", workspace=AgentWorkspace(article_id=23, version_no=7), blog_client=blog,
        )
    assert isinstance(result, AgentResponse)
    assert result.proposal.model_dump() == {
        "article_id": 23, "base_version_no": 7, "proposed_content": proposed_html(),
        "change_summary": ["改写介绍段，保留其余段落"],
    }
    assert len(calls) == 2
    assert json.loads(model.inputs[1][-1].content)["data"] == version_data()
    assert proposed_html().endswith("<p>保留段落</p>")
    # 模型自己生成的伪造 access_token 字段会留在消息历史中，但真实凭据不能出现。
    assert "actual-token" not in json.dumps([[m.model_dump(mode="json") for m in messages] for messages in model.inputs])
    for method in ("create_draft", "update_draft", "publish_article", "archive_article"):
        getattr(blog, method).assert_not_called()


@pytest.mark.anyio
async def test_no_workspace_chat_and_editing_fail_safely(settings):
    model = ScriptedModel([
        AIMessage(content="你好"), tool_call("read_workspace_article", {}),
        submission(article_id=23, version_no=7), AIMessage(content="没有活动文章工作区，请先选择文章及版本。"),
    ])
    runner = AgentRunner(model, settings=settings)

    def handler(request):
        pytest.fail("No workspace must not access Go for editing")

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as blog:
        chat = await runner.run_with_response("你好", access_token="fake-token", blog_client=blog)
        edit = await runner.run_with_response("改写文章 23 第 7 版", access_token="fake-token", blog_client=blog)
    assert chat.model_dump() == {"answer": "你好", "proposal": None}
    assert edit.proposal is None and "没有活动文章工作区" in edit.answer
    assert json.loads(model.inputs[2][-1].content)["error"] == "workspace_required"
    assert json.loads(model.inputs[3][-1].content)["error"] == "workspace_required"


@pytest.mark.anyio
@pytest.mark.parametrize("status,error", [(401, "authentication_required"), (403, "permission_denied"), (404, "article_not_found")])
@pytest.mark.parametrize("at_submission", [False, True])
async def test_authorization_or_missing_snapshot_blocks_proposal(settings, status, error, at_submission):
    calls = []

    def handler(request):
        calls.append(request)
        if at_submission and len(calls) == 1:
            return httpx.Response(200, json={"code": 200, "data": version_data()})
        return httpx.Response(status, json={"code": status, "message": "private-token private-backend-details"})

    replies = [tool_call("read_workspace_article", {})]
    if at_submission: replies.append(submission())
    replies.append(AIMessage(content="无法生成提案"))
    model = ScriptedModel(replies)
    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as blog:
        result = await AgentRunner(model, settings=settings).run_with_response(
            "改写", access_token="private-token", workspace=AgentWorkspace(article_id=23, version_no=7), blog_client=blog,
        )
    assert result.proposal is None
    tool_result = json.loads(model.inputs[-1][-1].content)
    assert tool_result["error"] == error
    assert "private-backend-details" not in json.dumps(tool_result)
    assert len(calls) == (2 if at_submission else 1)
    assert_no_token_in_model_inputs(model, "private-token")


@pytest.mark.anyio
@pytest.mark.parametrize("parallel_read", [False, True])
async def test_submit_requires_canonical_result_from_a_previous_model_turn(settings, parallel_read):
    calls = []

    def handler(request):
        calls.append(request)
        return httpx.Response(200, json={"code": 200, "data": version_data()})

    first = submission()
    if parallel_read:
        first = AIMessage(content="", tool_calls=[*tool_call("read_workspace_article", {}, "read").tool_calls, *first.tool_calls])
    model = ScriptedModel([first, AIMessage(content="需要先阅读完整版本")])
    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as blog:
        result = await AgentRunner(model, settings=settings).run_with_response(
            "改写", access_token="fake-token", workspace=AgentWorkspace(article_id=23, version_no=7), blog_client=blog,
        )
    assert result.proposal is None
    assert len(calls) == (1 if parallel_read else 0)
    submitted = next(m for m in model.inputs[-1] if isinstance(m, ToolMessage) and m.name == "submit_article_edit_proposal")
    assert json.loads(submitted.content)["error"] == "canonical_read_required"


@pytest.mark.anyio
async def test_proposal_run_cannot_execute_mutations_even_when_model_requests_them(settings):
    names = ["create_draft", "update_draft", "publish_article", "archive_article", "index_article_version"]
    model = ScriptedModel([
        AIMessage(content="", tool_calls=[{"name": name, "args": {"article_id": 23}, "id": name} for name in names]),
        AIMessage(content="此入口只生成提案"),
    ])

    def handler(request):
        pytest.fail("A forbidden tool reached Go")

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as blog:
        result = await AgentRunner(model, settings=settings).run_with_response(
            "保存并发布", access_token="fake-token", workspace=AgentWorkspace(article_id=23, version_no=7), blog_client=blog,
        )
    assert result.proposal is None
    replies = [m for m in model.inputs[1] if isinstance(m, ToolMessage)]
    assert len(replies) == len(names) and all(m.status == "error" for m in replies)
    assert not set(names) & {t.name for t in model.bound_tools}


class EditingModel(ScriptedModel):
    def __init__(self):
        super().__init__([])

    async def ainvoke(self, messages):
        self.inputs.append(list(messages))
        if isinstance(messages[-1], HumanMessage):
            if messages[-1].content == "hello": return AIMessage(content="hello")
            return tool_call("read_workspace_article", {}, "read")
        if messages[-1].name == "read_workspace_article":
            data = json.loads(messages[-1].content)["data"]
            return submission(proposed_content=data["content"] + "<p>新增结论</p>")
        return AIMessage(content="已生成编辑提案，尚未保存。")


@pytest.mark.anyio
async def test_one_runner_isolates_concurrent_workspaces_tokens_and_proposals(settings):
    model = EditingModel()
    runner = AgentRunner(model, settings=settings)
    calls = []

    async def handler(request):
        token = request.headers["Authorization"]
        article, version = (23, 7) if token == "Bearer token-A" else (24, 9)
        assert request.url.path == f"/api/v1/agent/articles/{article}/versions/{version}"
        assert request.method == "GET"
        calls.append((article, version, token))
        await asyncio.sleep(0)
        data = version_data(article, version)
        data["content"] = f"<p>Owner of {article}</p>"
        return httpx.Response(200, json={"code": 200, "data": data})

    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as blog:
        a, b = await asyncio.gather(*[
            runner.run_with_response("增加结论", access_token=token, workspace=workspace, blog_client=blog)
            for token, workspace in [("token-A", AgentWorkspace(article_id=23, version_no=7)), ("token-B", AgentWorkspace(article_id=24, version_no=9))]
        ])
        chat = await runner.run_with_response("hello", access_token="token-A", blog_client=blog)
    assert (a.proposal.article_id, a.proposal.base_version_no) == (23, 7)
    assert (b.proposal.article_id, b.proposal.base_version_no) == (24, 9)
    assert "Owner of 23" in a.proposal.proposed_content and "Owner of 24" not in a.proposal.proposed_content
    assert "Owner of 24" in b.proposal.proposed_content and "Owner of 23" not in b.proposal.proposed_content
    assert chat.proposal is None and len(calls) == 4
    for token in ("token-A", "token-B"): assert_no_token_in_model_inputs(model, token)


@pytest.mark.anyio
async def test_multiple_submissions_capture_only_first_success(settings):
    model = ScriptedModel([
        tool_call("read_workspace_article", {}), submission(), submission(proposed_content="<p>第二份</p>"),
        AIMessage(content="本次提案已生成"),
    ])
    async with BlogClient(settings, transport=httpx.MockTransport(lambda r: httpx.Response(200, json={"code": 200, "data": version_data()}))) as blog:
        result = await AgentRunner(model, settings=settings).run_with_response(
            "改写", access_token="fake-token", workspace=AgentWorkspace(article_id=23, version_no=7), blog_client=blog,
        )
    assert result.proposal.proposed_content == proposed_html()
    assert json.loads(model.inputs[-1][-1].content)["error"] == "proposal_already_submitted"


@pytest.mark.anyio
@pytest.mark.parametrize("fast_submission", [0, 1])
async def test_parallel_submissions_are_all_rejected_independent_of_completion_order(settings, fast_submission):
    requests = []
    both_started = asyncio.Event()
    fast_completed = asyncio.Event()

    async def handler(request):
        requests.append(request)
        ordinal = len(requests) - 2
        if ordinal >= 0:
            # 旧实现会进入两次提交校验；交替让第一/第二个请求先完成以暴露竞争。
            if len(requests) == 3:
                both_started.set()
            await asyncio.wait_for(both_started.wait(), 2)
            if ordinal == fast_submission:
                fast_completed.set()
            else:
                await asyncio.wait_for(fast_completed.wait(), 2)
        return httpx.Response(200, json={"code": 200, "data": version_data()})

    calls = [
        {"name": "submit_article_edit_proposal", "id": f"proposal-{i}", "args": {
            "proposed_content": f"<p>Proposal {i}</p>", "change_summary": [f"Change {i}"],
        }} for i in range(2)
    ]
    model = ScriptedModel([
        tool_call("read_workspace_article", {}), AIMessage(content="", tool_calls=calls),
        AIMessage(content="每轮只能提交一份提案，本轮未接受提案。"),
    ])
    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as blog:
        result = await AgentRunner(model, settings=settings).run_with_response(
            "编辑", access_token="fake-token", workspace=AgentWorkspace(article_id=23, version_no=7), blog_client=blog,
        )
    assert result.proposal is None
    replies = [m for m in model.inputs[-1] if isinstance(m, ToolMessage) and m.name == "submit_article_edit_proposal"]
    assert len(replies) == 2
    assert all(json.loads(m.content)["error"] == "multiple_proposal_submissions" for m in replies)
    assert len(requests) == 1  # 并行提交在发送 Go 校验请求前就被全部拒绝。


@pytest.mark.anyio
async def test_failure_after_submission_does_not_leak_capture_or_read_guard_to_next_run(settings):
    requests = []

    def handler(request):
        requests.append(request)
        assert request.url.path == "/api/v1/agent/articles/23/versions/7"
        assert request.headers["Authorization"] == "Bearer failed-run-token"
        return httpx.Response(200, json={"code": 200, "data": version_data()})

    model = ScriptedModel([
        tool_call("read_workspace_article", {}), submission(), RuntimeError("model failed after submission"),
        submission(), AIMessage(content="需要先读取本次工作区版本"),
        AIMessage(content="hello"), AIMessage(content="legacy hello"),
    ])
    runner = AgentRunner(model, settings=settings)
    async with BlogClient(settings, transport=httpx.MockTransport(handler)) as blog:
        with pytest.raises(AgentExecutionError):
            await runner.run_with_response(
                "编辑 A", access_token="failed-run-token", workspace=AgentWorkspace(article_id=23, version_no=7), blog_client=blog,
            )
        # 先证明失败发生前已经完成提案提交，而不是前置读取失败。
        assert json.loads(model.inputs[2][-1].content)["data"]["proposed_content"] == proposed_html()
        next_run = await runner.run_with_response(
            "编辑 B", access_token="next-run-token", workspace=AgentWorkspace(article_id=24, version_no=9), blog_client=blog,
        )
        chat = await runner.run_with_response("hello", access_token="next-run-token", blog_client=blog)
        legacy = await runner.run("hello", access_token="next-run-token", blog_client=blog)
    assert next_run.proposal is None
    assert json.loads(model.inputs[4][-1].content)["error"] == "canonical_read_required"
    assert not any(isinstance(m, ToolMessage) for m in model.inputs[3])
    assert chat.model_dump() == {"answer": "hello", "proposal": None}
    assert isinstance(legacy, AIMessage) and legacy.content == "legacy hello"
    assert len(requests) == 2


@pytest.mark.anyio
async def test_response_entry_reuses_rag_tool_without_using_it_as_editing_base(settings):
    hit = RetrievedChunk(article_id=23, version_no=7, chunk_index=2, heading_path=[], text="Retrieved context only", score=.8)
    rag = Mock(search_article_version=AsyncMock(return_value=[hit]))
    model = ScriptedModel([
        tool_call("search_article_version", {"article_id": 23, "version_no": 7, "query": "Redis"}),
        submission(), AIMessage(content="检索文本不能代替完整原文，需要先读取工作区版本。"),
    ])
    async with BlogClient(settings, transport=httpx.MockTransport(lambda r: pytest.fail("Unexpected Go access"))) as blog:
        result = await AgentRunner(model, settings=settings, rag_service=rag).run_with_response(
            "检索并编辑", access_token="fake-token", workspace=AgentWorkspace(article_id=23, version_no=7), blog_client=blog,
        )
        rag.search_article_version.assert_awaited_once_with(23, 7, "Redis", access_token="fake-token", blog_client=blog)
    assert result.proposal is None
    assert "Retrieved context only" in json.loads(model.inputs[1][-1].content)["data"]["context"]
    assert json.loads(model.inputs[-1][-1].content)["error"] == "canonical_read_required"


@pytest.mark.anyio
async def test_response_entry_uses_same_model_and_graph_topology(settings, monkeypatch):
    model = ScriptedModel([AIMessage(content="hello"), AIMessage(content="hello")])
    factory = Mock(return_value=model)
    monkeypatch.setattr("app.agent.runner.create_model", factory)
    runner = AgentRunner(settings=settings)
    for _ in range(2):
        await runner.run_with_response("hello", access_token="fake-token")
    factory.assert_called_once_with(settings)
    assert model.bind_count == 2  # 旧白名单和提案白名单各绑定一次，使用同一个模型实例。
    edges = lambda graph: {(e.source, e.target) for e in graph.get_graph().edges}
    assert edges(runner.graph) == edges(runner._proposal_graph)


def test_workspace_and_proposal_schemas_hide_identity_from_model():
    by_name = {t.name: t for t in PROPOSAL_TOOLS}
    assert set(by_name["read_workspace_article"].tool_call_schema.model_json_schema()["properties"]) == set()
    schema = by_name["submit_article_edit_proposal"].tool_call_schema.model_json_schema()
    assert set(schema["properties"]) == {"proposed_content", "change_summary"}
    for hidden in ("JWT", "access_token", "user_id", "article_id", "version_no", "runtime"):
        assert hidden not in json.dumps(schema)
    assert set(ArticleEditProposal.model_fields) == {"article_id", "base_version_no", "proposed_content", "change_summary"}
    with pytest.raises(ValidationError): AgentWorkspace(article_id=0, version_no=1)
    with pytest.raises(ValidationError): AgentWorkspace(article_id=1, version_no=True)
    with pytest.raises(ValidationError): AgentWorkspace(article_id=1, version_no=1, user_id=3)


@pytest.mark.parametrize("html", [
    "", "  ", "plain text", "```html\n<p>text</p>\n```", '<p>text', '<p>text</h1>',
    "<p></p>", "<p>&nbsp;</p>", "<!-- comment -->", '<script>alert(1)</script>',
    '<p onclick="alert(1)">text</p>', '<img src>', '{"replace":"paragraph"}',
])
def test_empty_or_invalid_html_proposal_rejected(html):
    with pytest.raises(ValidationError):
        ArticleEditProposal(article_id=23, base_version_no=7, proposed_content=html, change_summary=["改写"])


@pytest.mark.parametrize("summary", [[], [" "], [1], "changed"])
def test_invalid_change_summary_rejected(summary):
    with pytest.raises(ValidationError):
        ArticleEditProposal(article_id=23, base_version_no=7, proposed_content="<p>text</p>", change_summary=summary)


@pytest.mark.parametrize("html", [
    '<h1>标题</h1><p>内容<strong>加粗</strong></p><p>结论</p>',
    '<blockquote><p>引用</p></blockquote><ul><li><p>条目</p></li></ul>',
    '<pre><code>if x &lt; 2:\n    print(x)</code></pre>',
    '<p>图文</p><img src="/images/test.png"><p>尾段<br>换行</p>',
])
def test_complete_html_is_preserved_without_sanitizing_or_rewriting(html):
    proposal = ArticleEditProposal(article_id=23, base_version_no=7, proposed_content=html, change_summary=["调整结构"])
    assert proposal.proposed_content == html


@pytest.mark.anyio
async def test_invalid_submission_cannot_populate_capture(settings):
    model = ScriptedModel([tool_call("read_workspace_article", {}), submission(proposed_content="<p>unclosed"), AIMessage(content="提案 HTML 无效")])
    async with BlogClient(settings, transport=httpx.MockTransport(lambda r: httpx.Response(200, json={"code": 200, "data": version_data()}))) as blog:
        result = await AgentRunner(model, settings=settings).run_with_response(
            "编辑", access_token="fake-token", workspace=AgentWorkspace(article_id=23, version_no=7), blog_client=blog,
        )
    assert result.proposal is None
    assert json.loads(model.inputs[-1][-1].content)["error"] == "invalid_arguments"
