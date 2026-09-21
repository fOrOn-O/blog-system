import json

import groq
import httpx
import pytest

from app.agent.models import AgentWorkspace
from app.agent.runner import AgentRunner
from app.clients.blog import BlogClient
from app.core.config import Settings
from app.agent.errors import AgentExecutionError
from langchain_core.messages import AIMessage
from test_agent import ScriptedModel, tool_call, assert_no_token_in_model_inputs
from test_article_version_client import version_data


@pytest.fixture
def anyio_backend():
    return "asyncio"


@pytest.mark.anyio
async def test_full_article_structure_proposal_survives_without_post_submission_model_call(monkeypatch):
    monkeypatch.setenv("LANGSMITH_TRACING", "false")
    monkeypatch.setenv("LANGCHAIN_TRACING_V2", "false")
    settings = Settings(_env_file=None, groq_api_key="")
    snapshot = version_data()
    snapshot["content"] = "<p>开篇说明</p>" + "<p>正文证据与细节。</p>" * 400 + "<p>原文末段</p>"
    proposed = "<h2>背景与目的</h2>" + snapshot["content"] + "<h2>总结</h2><p>保留全文，调整结构。</p>"
    # 真实复现中第三次模型请求在提案已通过校验后触发上游 429。
    failure = groq.RateLimitError("private-provider-detail", response=httpx.Response(429,
        request=httpx.Request("POST", "https://model.test/chat")), body={})
    model = ScriptedModel([
        tool_call("read_workspace_article", {}),
        tool_call("submit_article_edit_proposal", {"proposed_content": proposed, "change_summary": ["整体优化正文结构"]}),
        failure,
    ])
    calls = []
    def source(request):
        assert request.method == "GET"
        assert request.url.path == "/api/v1/agent/articles/23/versions/7"
        calls.append(request)
        return httpx.Response(200, json={"code": 200, "data": snapshot})
    async with BlogClient(settings, transport=httpx.MockTransport(source)) as blog:
        result = await AgentRunner(model, settings=settings).run_with_response(
            "帮我整体优化一下正文结构", access_token="runtime-test-token", mode="write",
            workspace=AgentWorkspace(article_id=23, version_no=7), blog_client=blog)
    assert result.proposal.proposed_content == proposed
    assert json.loads(model.inputs[1][-1].content)["data"]["content"] == snapshot["content"]
    assert len(model.inputs) == 2 and len(calls) == 2
    assert "预览" in result.answer and "确认" in result.answer and "尚未保存" in result.answer
    assert all(name not in result.answer for name in ("list_my_articles", "submit_article_edit_proposal", "search_article_version"))
    assert_no_token_in_model_inputs(model, "runtime-test-token")


@pytest.mark.anyio
@pytest.mark.parametrize("kind,code,status", [("rate", "model_rate_limited", 502),
    ("timeout", "model_timeout", 504), ("request", "model_request_rejected", 502),
    ("invalid_json", "model_output_invalid", 502), ("truncated", "model_output_invalid", 502)])
async def test_model_failures_are_classified_without_retry_or_sensitive_logging(monkeypatch, caplog, kind, code, status):
    monkeypatch.setenv("LANGSMITH_TRACING", "false")
    request = httpx.Request("POST", "https://model.test")
    responses = {
        "rate": groq.RateLimitError("private-jwt secret-body", response=httpx.Response(429, request=request), body={}),
        "timeout": groq.APITimeoutError(request=request),
        "request": groq.BadRequestError("private-jwt secret-body", response=httpx.Response(400, request=request), body={}),
        "invalid_json": AIMessage(content="", invalid_tool_calls=[{"name": "submit_article_edit_proposal", "args": "{", "id": "broken", "error": "bad json"}]),
        "truncated": AIMessage(content="<p>partial", response_metadata={"finish_reason": "length"}),
    }
    model = ScriptedModel([responses[kind]])
    with pytest.raises(AgentExecutionError) as caught:
        await AgentRunner(model, settings=Settings(_env_file=None, groq_api_key="")).run_with_response(
            "帮我整体优化一下正文结构", access_token="private-jwt", workspace=AgentWorkspace(article_id=23, version_no=7))
    assert (caught.value.code, caught.value.status_code) == (code, status)
    assert len(model.inputs) == 1
    assert "secret-body" not in caplog.text and "private-jwt" not in caplog.text


@pytest.mark.anyio
async def test_product_language_policy_reaches_actual_model_paths(monkeypatch):
    monkeypatch.setenv("LANGSMITH_TRACING", "false")
    from app.knowledge.answer import grounded_answer
    from app.knowledge.models import KnowledgeHit
    from app.agent.prompts import RESPONSE_POLICY
    model = ScriptedModel([AIMessage(content="可以查看文章与生成编辑建议。") for _ in range(3)])
    runner = AgentRunner(model, settings=Settings(_env_file=None, groq_api_key=""))
    await runner.run("你有什么功能？", access_token="test-token")
    await runner.run_with_response("你有什么功能？", access_token="test-token")
    await grounded_answer("内容是什么？", [KnowledgeHit(article_id=23, version_no=7, title="标题", chunk_index=0,
        heading_path=[], text="证据", score=.8)], model_factory=lambda: model, scope="当前版本")
    for messages in model.inputs:
        assert any(RESPONSE_POLICY in m.content for m in messages if m.type == "system")
