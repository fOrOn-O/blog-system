from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, HumanMessage
from langgraph.errors import GraphRecursionError

from app.agent.context import AgentContext, ProposalCapture
from app.agent.errors import AgentExecutionError
from app.agent.graph import build_graph
from app.agent.models import AgentResponse, AgentWorkspace
from app.agent.prompts import PROPOSAL_SYSTEM_PROMPT
from app.agent.proposal_tools import PROPOSAL_TOOLS
from app.clients.blog import BlogClient
from app.core.config import Settings, get_settings
from app.core.model import create_model
from app.rag.service import ArticleRagService


class AgentRunner:
    """Reuse the compiled graph/model; each run has fresh messages and credentials."""

    def __init__(self, model: BaseChatModel | None = None, *, settings: Settings | None = None, rag_service: ArticleRagService | None = None):
        self.settings = settings if settings is not None else get_settings()
        self.rag_service = rag_service
        self._model = model if model is not None else create_model(self.settings)
        self.graph = build_graph(self._model)
        self._proposal_graph = None

    async def run(
        self, message: str, *, access_token: str, blog_client: BlogClient | None = None
    ) -> AIMessage:
        self._validate_input(message, access_token)
        if blog_client is not None:
            return await self._invoke(message, AgentContext(access_token, blog_client, self.rag_service))
        async with BlogClient(self.settings) as client:
            return await self._invoke(message, AgentContext(access_token, client, self.rag_service))

    @staticmethod
    def _validate_input(message: str, access_token: str) -> None:
        if not isinstance(message, str) or not message.strip():
            raise ValueError("message must not be empty")
        if not isinstance(access_token, str) or not access_token or any(c.isspace() for c in access_token):
            raise ValueError("access_token must be a nonempty token without whitespace")

    async def run_with_response(
        self, message: str, *, access_token: str, workspace: AgentWorkspace | None = None,
        blog_client: BlogClient | None = None,
    ) -> AgentResponse:
        """只读问答/提案入口；保留旧 run() 的返回类型和工具行为。"""
        self._validate_input(message, access_token)
        if workspace is not None and not isinstance(workspace, AgentWorkspace):
            raise ValueError("workspace must be an AgentWorkspace")
        if self._proposal_graph is None:
            self._proposal_graph = build_graph(self._model, tools=PROPOSAL_TOOLS, system_prompt=PROPOSAL_SYSTEM_PROMPT)
        capture = ProposalCapture()

        async def invoke(client: BlogClient) -> AgentResponse:
            context = AgentContext(access_token, client, self.rag_service, workspace, capture)
            final = await self._invoke(message, context, graph=self._proposal_graph)
            return AgentResponse(answer=final.text, proposal=capture.proposal)

        if blog_client is not None:
            return await invoke(blog_client)
        async with BlogClient(self.settings) as client:
            return await invoke(client)

    async def _invoke(self, message: str, context: AgentContext, *, graph=None) -> AIMessage:
        try:
            result = await (graph if graph is not None else self.graph).ainvoke(
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
