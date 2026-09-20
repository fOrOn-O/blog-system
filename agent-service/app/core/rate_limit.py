import math
import time
from collections import deque
from functools import lru_cache
from threading import Lock
from typing import Protocol

from fastapi import HTTPException

from app.core.config import get_settings


class Limiter(Protocol):
    def retry_after(self, subject: tuple[str, int | str]) -> int: ...


class RequestLimiter:
    """进程内滑动窗口。主体使用命名空间；未来 IP 入口可以独立提供可信的 ('ip', value)。"""
    def __init__(self, requests: int, window_seconds: int, *, clock=time.monotonic, max_subjects: int = 10000):
        self.requests, self.window_seconds, self.clock = requests, window_seconds, clock
        self.max_subjects = max_subjects
        self._buckets = {}
        self._lock = Lock()

    def retry_after(self, subject: tuple[str, int | str]) -> int:
        with self._lock:
            now = self.clock()
            cutoff = now - self.window_seconds
            expired = [key for key, stamps in self._buckets.items() if stamps[-1] <= cutoff]
            for key in expired:
                del self._buckets[key]
            if subject not in self._buckets and len(self._buckets) >= self.max_subjects:
                return self.window_seconds
            stamps = self._buckets.setdefault(subject, deque())
            while stamps and stamps[0] <= cutoff:
                stamps.popleft()
            if len(stamps) >= self.requests:
                return max(1, math.ceil(stamps[0] + self.window_seconds - now))
            stamps.append(now)
            return 0


@lru_cache
def get_request_limiter() -> Limiter:
    settings = get_settings()
    return RequestLimiter(settings.chat_rate_limit_requests, settings.chat_rate_limit_window_seconds)


def limit_user(user_id: int) -> None:
    # ID 仅由 Go 认证 profile 提供，不解码未验证 JWT、不接受用户传入的 subject。
    retry = get_request_limiter().retry_after(("user", user_id))
    if retry:
        raise HTTPException(429, detail={"code": "rate_limit_exceeded", "message": "请求过于频繁，请稍后再试。"},
                            headers={"Retry-After": str(retry)})
