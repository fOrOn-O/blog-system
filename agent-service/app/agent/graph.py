from typing import Literal

from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, SystemMessage
from langgraph.graph import END, START, MessagesState, StateGraph

from langgraph.prebuilt import ToolNode
from langgraph.runtime import Runtime

from app.agent.context import AgentContext
from app.agent.errors import AgentExecutionError
from app.agent.prompts import SYSTEM_PROMPT
from app.agent.tools import AGENT_TOOLS, safe_tool_error


def build_graph(model: BaseChatModel, *, tools=AGENT_TOOLS, system_prompt: str = SYSTEM_PROMPT):
    model_with_tools = model.bind_tools(list(tools))

    async def call_model(state: MessagesState, runtime: Runtime[AgentContext]) -> dict:
        instructions = [SystemMessage(content=system_prompt)]
        if runtime.context.proposal_capture is not None:
            workspace_note = (
                "当前有活动文章工作区；使用 read_workspace_article 读取绑定的版本。"
                if runtime.context.workspace is not None else
                "当前没有活动文章工作区；可以正常问答，但不能编辑或提交文章提案。"
            )
            instructions.append(SystemMessage(content=workspace_note))
        reply = await model_with_tools.ainvoke([*instructions, *state["messages"]])
        if not isinstance(reply, AIMessage):
            raise AgentExecutionError("Model returned an invalid agent response")
        return {"messages": [reply]}

    def should_continue(state: MessagesState) -> Literal["tools", "__end__"]:
        return "tools" if state["messages"][-1].tool_calls else END

    graph = StateGraph(MessagesState, context_schema=AgentContext)
    graph.add_node("call_model", call_model)
    graph.add_node("tools", ToolNode(tools, handle_tool_errors=safe_tool_error))
    graph.add_edge(START, "call_model")
    graph.add_conditional_edges("call_model", should_continue, {"tools": "tools", END: END})
    graph.add_edge("tools", "call_model")
    return graph.compile()
