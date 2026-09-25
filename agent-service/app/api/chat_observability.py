"""只为 Chat 请求生成诊断 ID，不接受调用方提交的任意日志标识。"""

from uuid import uuid4

from fastapi import HTTPException, Response

from app.core.agent_observability import record, request_id


async def observe_chat(response: Response):
    identity = uuid4().hex
    token = request_id.set(identity)
    response.headers["X-Request-ID"] = identity
    record("request")
    try:
        yield
    except HTTPException as error:
        error.headers = {**(error.headers or {}), "X-Request-ID": identity}
        raise
    finally:
        request_id.reset(token)
