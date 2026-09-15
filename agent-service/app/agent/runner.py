from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, HumanMessage
from langgraph.errors import GraphRecursionError

from app.agent.context import AgentContext
from app.agent.errors import AgentExecutionError
from app.agent.graph import build_graph
from app.clients.blog import BlogClient
from app.core.config import Settings, get_settings
from app.core.model import create_model


class AgentRunner:
    """Reuse the compiled graph/model; each run has fresh messages and credentials."""

    def __init__(self, model: BaseChatModel | None = None, *, settings: Settings | None = None):
        self.settings = settings if settings is not None else get_settings()
        self.graph = build_graph(model if model is not None else create_model(self.settings))

    async def run(
        self, message: str, *, access_token: str, blog_client: BlogClient | None = None
    ) -> AIMessage:
        if not isinstance(message, str) or not message.strip():
            raise ValueError("message must not be empty")
        if not isinstance(access_token, str) or not access_token or any(c.isspace() for c in access_token):
            raise ValueError("access_token must be a nonempty token without whitespace")
        if blog_client is not None:
            return await self._invoke(message, AgentContext(access_token, blog_client))
        async with BlogClient(self.settings) as client:
            return await self._invoke(message, AgentContext(access_token, client))

    async def _invoke(self, message: str, context: AgentContext) -> AIMessage:
        try:
            result = await self.graph.ainvoke(
                {"messages": [HumanMessage(content=message)]},
                context=context,
                config={"recursion_limit": self.settings.agent_recursion_limit},
            )
        except GraphRecursionError:
            raise AgentExecutionError("Agent reached the execution step limit") from None
        except Exception:
            raise AgentExecutionError("Agent execution failed") from None
        final = result["messages"][-1]
        if not isinstance(final, AIMessage) or final.tool_calls:
            raise AgentExecutionError("Agent did not produce a final answer")
        return final
