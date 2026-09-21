import logging

import groq

from app.agent.errors import AgentExecutionError

logger = logging.getLogger(__name__)


def model_failure(error: Exception) -> AgentExecutionError:
    # 只记录受控分类，不记录异常原文、请求/响应 body、token 或 traceback。
    code, status = "model_response_failed", 502
    if isinstance(error, groq.RateLimitError):
        code = "model_rate_limited"
    elif isinstance(error, (groq.APITimeoutError, TimeoutError)):
        code, status = "model_timeout", 504
    elif isinstance(error, groq.BadRequestError):
        code = "model_request_rejected"
    elif isinstance(error, groq.APIConnectionError):
        code = "model_unavailable"
    logger.warning("agent_failure stage=model code=%s", code)
    return AgentExecutionError(code=code, status_code=status)
