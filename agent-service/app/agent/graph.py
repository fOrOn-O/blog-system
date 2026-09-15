from typing import Literal

from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, SystemMessage
from langgraph.graph import END, START, MessagesState, StateGraph

from langgraph.prebuilt import ToolNode

from app.agent.context import AgentContext
from app.agent.errors import AgentExecutionError
from app.agent.prompts import SYSTEM_PROMPT
from app.agent.tools import AGENT_TOOLS, safe_tool_error


def build_graph(model: BaseChatModel):
    model_with_tools = model.bind_tools(list(AGENT_TOOLS))

    async def call_model(state: MessagesState) -> dict:
        reply = await model_with_tools.ainvoke(
            [SystemMessage(content=SYSTEM_PROMPT), *state["messages"]]
        )
        if not isinstance(reply, AIMessage):
            raise AgentExecutionError("Model returned an invalid agent response")
        return {"messages": [reply]}

    def should_continue(state: MessagesState) -> Literal["tools", "__end__"]:
        return "tools" if state["messages"][-1].tool_calls else END

    graph = StateGraph(MessagesState, context_schema=AgentContext)
    graph.add_node("call_model", call_model)
    graph.add_node("tools", ToolNode(AGENT_TOOLS, handle_tool_errors=safe_tool_error))
    graph.add_edge(START, "call_model")
    graph.add_conditional_edges("call_model", should_continue, {"tools": "tools", END: END})
    graph.add_edge("tools", "call_model")
    return graph.compile()
