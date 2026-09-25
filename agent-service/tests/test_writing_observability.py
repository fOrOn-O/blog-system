"""真实 SDK 解析 + 内存 HTTP：验证诊断信息，不调用外部模型或数据库。"""

import asyncio
from contextlib import asynccontextmanager
import json
import logging
from uuid import UUID

import httpx
import pytest
from fastapi import FastAPI
from langchain_groq import ChatGroq

from app.agent.runner import AgentRunner
from app.api import chat
from app.clients.blog import BlogClient
from app.core.agent_observability import request_id
from app.core.config import Settings
from test_article_version_client import version_data

READ = "read_workspace_article"
SUBMIT = "submit_article_edit_proposal"
PRIVATE_TOKEN = "PRIVATE_JWT_DO_NOT_LOG"
PRIVATE_KEY = "PRIVATE_API_KEY_DO_NOT_LOG"
PRIVATE_PROMPT = "PRIVATE_PROMPT_DO_NOT_LOG"
PRIVATE_HTML = "<p>PRIVATE_PROPOSAL_HTML_DO_NOT_LOG</p>"
ARGS = {"proposed_content": PRIVATE_HTML, "change_summary": ["PRIVATE_SUMMARY_DO_NOT_LOG"]}


@pytest.fixture
def anyio_backend():
    return "asyncio"


def tool_call(name, args=None, *, raw=None):
    return {"id": "call_" + name, "type": "function", "function": {
        "name": name, "arguments": raw if raw is not None else json.dumps(args or {}),
    }}


def reply(content="", calls=None):
    message = {"role": "assistant", "content": content}
    if calls is not None:
        message["tool_calls"] = calls
    return message


def events(caplog, identity=None):
    result = [json.loads(record.getMessage().removeprefix("agent_observation "))
              for record in caplog.records if record.name == "app.core.agent_observability"]
    return [event for event in result if identity is None or event["request_id"] == identity]


@pytest.fixture
def harness(monkeypatch, caplog):
    monkeypatch.setenv("LANGSMITH_TRACING", "false")
    monkeypatch.setenv("LANGCHAIN_TRACING_V2", "false")
    caplog.set_level(logging.INFO)
    settings = Settings(_env_file=None, groq_api_key=PRIVATE_KEY,
                        blog_backend_url="http://mock-go.invalid")
    monkeypatch.setattr(chat, "get_settings", lambda: settings)
    monkeypatch.setattr(chat, "limit_user", lambda _: None)

    @asynccontextmanager
    async def start(responses, *, bad_snapshot=None, finish_reason=None):
        calls = []
        version_reads = 0

        async def go(request):
            nonlocal version_reads
            assert request.method == "GET"
            assert request.headers["Authorization"] == "Bearer " + PRIVATE_TOKEN
            calls.append("go:" + request.url.path)
            await asyncio.sleep(0)
            if request.url.path == "/api/v1/user/profile":
                data = {"id": 7}
            else:
                version_reads += 1
                parts = request.url.path.split("/")
                data = version_data(int(parts[-3]), int(parts[-1]))
                data["content"] = "<p>PRIVATE_SOURCE_HTML_DO_NOT_LOG</p>"
                if version_reads == 2:
                    if bad_snapshot == "schema":
                        del data["content"]
                    elif bad_snapshot == "identity":
                        data["version_no"] += 1
                    elif bad_snapshot == "json":
                        return httpx.Response(200, text="PRIVATE_RESPONSE_DO_NOT_LOG")
            return httpx.Response(200, json={"code": 200, "data": data})

        async def groq(request):
            assert request.url.host == "mock-groq.invalid"
            body = json.loads(request.content)
            assert "tool_choice" not in body
            # 根据当前消息历史选取响应，使同一个 Runner 可以并发运行。
            turn = sum(message["role"] == "assistant" for message in body["messages"])
            assert turn < len(responses), "unexpected extra model request"
            calls.append("model:" + str(turn + 1))
            await asyncio.sleep(0)
            message = responses[turn]
            reason = finish_reason or ("tool_calls" if message.get("tool_calls") else "stop")
            return httpx.Response(200, json={
                "id": "mock-completion", "object": "chat.completion", "created": 1,
                "model": "openai/gpt-oss-20b",
                "choices": [{"index": 0, "message": message, "finish_reason": reason}],
            })

        monkeypatch.setattr(chat, "BlogClient", lambda _: BlogClient(settings, transport=httpx.MockTransport(go)))
        async with httpx.AsyncClient(transport=httpx.MockTransport(groq), trust_env=False) as model_http:
            model = ChatGroq(model="openai/gpt-oss-20b", api_key=PRIVATE_KEY, max_retries=0,
                             base_url="https://mock-groq.invalid/openai/v1", http_async_client=model_http)
            runner = AgentRunner(model, settings=settings)
            app = FastAPI()
            app.include_router(chat.router)
            app.dependency_overrides[chat.runner_factory] = lambda: lambda: runner
            async with httpx.AsyncClient(transport=httpx.ASGITransport(app=app), base_url="http://test") as client:
                yield client, calls

    yield start
    # 覆盖全部捕获日志，而不只检查新 logger；标记分别放在凭据、正文和错误响应里。
    for sensitive in (PRIVATE_TOKEN, PRIVATE_KEY, PRIVATE_PROMPT, PRIVATE_HTML,
                      "PRIVATE_SOURCE_HTML_DO_NOT_LOG", "PRIVATE_SUMMARY_DO_NOT_LOG",
                      "PRIVATE_RESPONSE_DO_NOT_LOG"):
        assert sensitive not in caplog.text


async def send(client, article_id=23, version_no=7):
    return await client.post("/api/v1/agent/chat", json={
        "mode": "write", "message": PRIVATE_PROMPT,
        "workspace": {"article_id": article_id, "version_no": version_no},
    }, headers={"Authorization": "Bearer " + PRIVATE_TOKEN, "X-Request-ID": PRIVATE_TOKEN})


@pytest.mark.anyio
@pytest.mark.parametrize("content", ["Ordinary answer", json.dumps(ARGS), PRIVATE_HTML, ""])
async def test_two_http200_replies_end_without_proposal(harness, caplog, content):
    async with harness([reply(calls=[tool_call(READ)]), reply(content)]) as (client, calls):
        response = await send(client)
    assert response.status_code == 502
    assert response.json()["detail"]["code"] == "proposal_not_created"
    identity = response.headers["X-Request-ID"]
    assert UUID(identity).version == 4
    trace = events(caplog, identity)
    assert len(trace) == len(events(caplog))
    models = [event for event in trace if event["stage"] == "model"]
    assert [event["model_round"] for event in models] == [1, 2]
    assert models[0]["tool_names"] == [READ]
    assert models[1] == {"request_id": identity, "stage": "model", "model_round": 2,
                         "finish_reason": "stop", "tool_call_count": 0, "tool_names": [],
                         "invalid_tool_call_count": 0, "content_empty": content == "",
                         "content_length": len(content)}
    assert not any(event["stage"] == "tool" for event in trace)
    assert trace[-2] == {"request_id": identity, "stage": "runner", "model_rounds": 2,
                         "canonical_present": True, "proposal_present": False}
    assert trace[-1]["reason"] == "proposal_missing"
    assert calls == ["go:/api/v1/user/profile", "go:/api/v1/agent/articles/23/versions/7",
                     "model:1", "go:/api/v1/agent/articles/23/versions/7", "model:2"]
    assert request_id.get() is None


@pytest.mark.anyio
async def test_prior_parallel_tool_failure_is_distinguishable(harness, caplog):
    responses = [reply(calls=[tool_call(READ), tool_call(SUBMIT, ARGS)]), reply("Cannot complete")]
    async with harness(responses) as (client, _):
        response = await send(client)
    assert response.status_code == 502
    trace = events(caplog, response.headers["X-Request-ID"])
    error = next(event for event in trace if event["stage"] == "tool")
    assert error["tool_name"] == SUBMIT and error["result"] == "error"
    assert error["reason"] == "canonical_read_required"
    assert trace.index(error) < next(i for i, event in enumerate(trace)
                                     if event["stage"] == "model" and event["model_round"] == 2)
    assert [event["proposal_stage"] for event in trace if "proposal_stage" in event] == ["entered"]


@pytest.mark.anyio
@pytest.mark.parametrize("bad_snapshot,expected", [("schema", (False, None)),
                                                   ("json", (False, None)), ("identity", (True, False))])
async def test_snapshot_validation_failure_is_classified(harness, caplog, bad_snapshot, expected):
    async with harness([reply(calls=[tool_call(READ)]), reply("Cannot read")],
                       bad_snapshot=bad_snapshot) as (client, _):
        response = await send(client)
    assert response.json()["detail"]["code"] == "proposal_not_created"
    trace = events(caplog)
    reads = [event for event in trace if event["stage"] == READ]
    assert (reads[-1]["response_validation"], reads[-1]["identity_validation"]) == expected
    assert reads[-1]["article_id"] == 23 and reads[-1]["version_no"] == 7
    assert next(event for event in trace if event["stage"] == "tool")["reason"] == "backend_error"
    assert next(event for event in trace if event["stage"] == "runner")["canonical_present"] is False


@pytest.mark.anyio
async def test_success_logs_all_stages_without_extra_model_call(harness, caplog):
    async with harness([reply(calls=[tool_call(READ)]), reply(calls=[tool_call(SUBMIT, ARGS)])]) as (client, calls):
        response = await send(client)
    assert response.status_code == 200
    assert response.json()["proposal"]["proposed_content"] == PRIVATE_HTML
    trace = events(caplog, response.headers["X-Request-ID"])
    assert [event["proposal_stage"] for event in trace if "proposal_stage" in event] == [
        "entered", "guard_passed", "canonical_recheck_passed", "validation_passed", "capture_written"]
    assert trace[-1]["proposal_present"] is True and trace[-1]["model_rounds"] == 2
    assert [event["response_validation"] for event in trace if event["stage"] == READ] == [None, True]
    assert calls.count("go:/api/v1/agent/articles/23/versions/7") == 3
    assert not any(event.get("reason") == "proposal_missing" for event in trace)


@pytest.mark.anyio
async def test_invalid_html_logs_last_passed_stage_without_html(harness, caplog):
    async with harness([reply(calls=[tool_call(READ)]),
                        reply(calls=[tool_call(SUBMIT, {**ARGS, "proposed_content": "<p>PRIVATE_RESPONSE_DO_NOT_LOG"})]),
                        reply("Invalid proposal")]) as (client, _):
        response = await send(client)
    assert response.json()["detail"]["code"] == "proposal_not_created"
    trace = events(caplog)
    assert [event["proposal_stage"] for event in trace if "proposal_stage" in event] == [
        "entered", "guard_passed", "canonical_recheck_passed"]
    assert next(event for event in trace if event["stage"] == "tool")["reason"] == "invalid_arguments"
    assert next(event for event in trace if event["stage"] == "runner")["model_rounds"] == 3


@pytest.mark.anyio
async def test_invalid_tool_json_keeps_existing_error_mapping(harness, caplog):
    async with harness([reply(calls=[tool_call(READ)]),
                        reply(calls=[tool_call(SUBMIT, raw="{PRIVATE_RESPONSE_DO_NOT_LOG")])]) as (client, _):
        response = await send(client)
    assert response.json()["detail"]["code"] == "model_output_invalid"
    assert [event for event in events(caplog) if event["stage"] == "model"][-1]["invalid_tool_call_count"] == 1
    assert not any(event.get("reason") == "proposal_missing" for event in events(caplog))


@pytest.mark.anyio
async def test_untrusted_tool_name_and_finish_reason_are_not_logged(harness, caplog):
    async with harness([reply(calls=[tool_call(PRIVATE_TOKEN)]), reply("Done")],
                       finish_reason=PRIVATE_KEY) as (client, _):
        response = await send(client)
    assert response.json()["detail"]["code"] == "proposal_not_created"
    trace = events(caplog)
    model = next(event for event in trace if event["stage"] == "model")
    assert model["tool_names"] == ["unregistered_tool"] and model["finish_reason"] == "other"
    error = next(event for event in trace if event["stage"] == "tool")
    assert error["reason"] == "unregistered_tool" and error["tool_name"] == "unregistered_tool"


@pytest.mark.anyio
@pytest.mark.parametrize("submit", [False, True])
async def test_concurrent_requests_share_runner_but_not_diagnostics(harness, caplog, submit):
    final = reply(calls=[tool_call(SUBMIT, ARGS)]) if submit else reply("Done")
    async with harness([reply(calls=[tool_call(READ)]), final]) as (client, _):
        first, second = await asyncio.gather(send(client, 23, 7), send(client, 24, 8))
    assert [response.status_code for response in (first, second)] == ([200, 200] if submit else [502, 502])
    identities = [response.headers["X-Request-ID"] for response in (first, second)]
    assert identities[0] != identities[1]
    for identity, article, version in zip(identities, (23, 24), (7, 8)):
        trace = events(caplog, identity)
        assert [event["model_round"] for event in trace if event["stage"] == "model"] == [1, 2]
        assert all(event["article_id"] == article and event["version_no"] == version
                   for event in trace if "article_id" in event)
        assert next(event for event in trace if event["stage"] == "runner")["model_rounds"] == 2
        assert next(event for event in trace if event["stage"] == "runner")["proposal_present"] is submit
    assert request_id.get() is None


@pytest.mark.anyio
async def test_tool_schema_failure_is_logged_before_submission_body(harness, caplog):
    async with harness([reply(calls=[tool_call(READ)]),
                        reply(calls=[tool_call(SUBMIT, {"change_summary": ["PRIVATE_SUMMARY_DO_NOT_LOG"]})]),
                        reply("Missing argument")]) as (client, _):
        response = await send(client)
    assert response.json()["detail"]["code"] == "proposal_not_created"
    trace = events(caplog)
    error = next(event for event in trace if event["stage"] == "tool")
    assert error["tool_name"] == SUBMIT and error["reason"] == "invalid_arguments"
    assert not any("proposal_stage" in event for event in trace)


@pytest.mark.anyio
async def test_early_auth_failure_has_own_id_and_does_not_leak(harness, caplog):
    async with harness([reply(calls=[tool_call(READ)]), reply("Done")]) as (client, calls):
        rejected = await client.post("/api/v1/agent/chat", json={
            "mode": "write", "message": PRIVATE_PROMPT, "workspace": {"article_id": 23, "version_no": 7},
        })
        assert rejected.status_code == 401 and not calls
        assert request_id.get() is None
        accepted = await send(client)
    first_id, second_id = (response.headers["X-Request-ID"] for response in (rejected, accepted))
    assert first_id != second_id
    assert events(caplog, first_id) == [{"request_id": first_id, "stage": "request"}]
    assert len(events(caplog, second_id)) > 1
