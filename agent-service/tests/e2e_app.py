"""浏览器 E2E 专用应用：只替换 LLM，使用真实 Router/Runner/ToolNode/BlogClient。"""
import json
import os

os.environ["LANGSMITH_TRACING"] = "false"
os.environ["LANGCHAIN_TRACING_V2"] = "false"

from fastapi import FastAPI
from langchain_core.messages import AIMessage, HumanMessage
from app.api import chat
from app.agent.runner import AgentRunner
from app.core.config import Settings


class WritingModel:
    def bind_tools(self, tools):
        return self

    async def ainvoke(self, messages):
        last = messages[-1]
        if isinstance(last, HumanMessage):
            if "你好" in last.content:
                return AIMessage(content="你好，可以围绕当前版本提问或提出修改。")
            return AIMessage(content="", tool_calls=[{"name": "read_workspace_article", "args": {}, "id": "read"}])
        if last.name == "read_workspace_article":
            content = json.loads(last.content)["data"]["content"]
            return AIMessage(content="", tool_calls=[{"name": "submit_article_edit_proposal", "args": {
                "proposed_content": content + "<p>助手补充的结论。</p>",
                "change_summary": ["保留原文，补充结论段"]}, "id": "proposal"}])
        return AIMessage(content="已生成编辑提案，尚未保存。")


settings = Settings(_env_file=None, groq_api_key="", blog_backend_url="http://127.0.0.1:18080")
chat.get_settings = lambda: settings
runner = AgentRunner(WritingModel(), settings=settings)
app = FastAPI()
app.include_router(chat.router)
app.dependency_overrides[chat.runner_factory] = lambda: lambda: runner


@app.get("/health")
def health():
    return {"status": "ok"}
