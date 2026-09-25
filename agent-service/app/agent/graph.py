from typing import Literal

from langchain_core.language_models import BaseChatModel
from langchain_core.messages import AIMessage, SystemMessage
from langgraph.graph import END, START, MessagesState, StateGraph

from langgraph.prebuilt import ToolNode
from langgraph.runtime import Runtime

from app.agent.context import AgentContext
from app.agent.errors import AgentExecutionError
from app.agent.model_errors import model_failure
from app.agent.prompts import SYSTEM_PROMPT
from app.agent.tools import AGENT_TOOLS, safe_tool_error
from app.core.agent_observability import model_returned, observe_tool_call


def build_graph(model: BaseChatModel, *, tools=AGENT_TOOLS, system_prompt: str = SYSTEM_PROMPT):
    model_with_tools = model.bind_tools(list(tools))

    async def call_model(state: MessagesState, runtime: Runtime[AgentContext]) -> dict:
        capture = runtime.context.proposal_capture
        if capture is not None and capture.proposal is not None:
            # 完整提案已校验并捕获，结束本次生成；不再将正文发回模型润色确认语。
            return {"messages": [AIMessage(content="已生成完整编辑提案，尚未保存。请先预览差异，再由你确认是否应用；不会自动发布。") ]}
        instructions = [SystemMessage(content=system_prompt)]
        if runtime.context.proposal_capture is not None:
            workspace_note = (
                "当前有活动文章工作区；使用 read_workspace_article 读取绑定的版本。"
                if runtime.context.workspace is not None else
                "当前没有活动文章工作区；可以正常问答，但不能编辑或提交文章提案。"
            )
            instructions.append(SystemMessage(content=workspace_note))
        try:
            reply = await model_with_tools.ainvoke([*instructions, *state["messages"]])
        except Exception as error:
            raise model_failure(error) from None
        if not isinstance(reply, AIMessage):
            raise AgentExecutionError("Model returned an invalid agent response")
        model_returned(reply, {tool.name for tool in tools})
        if reply.invalid_tool_calls or reply.response_metadata.get("finish_reason") == "length":
            raise AgentExecutionError(code="model_output_invalid")
        return {"messages": [reply]}

    def should_continue(state: MessagesState) -> Literal["tools", "__end__"]:
        return "tools" if state["messages"][-1].tool_calls else END

    graph = StateGraph(MessagesState, context_schema=AgentContext)
    graph.add_node("call_model", call_model)
    graph.add_node("tools", ToolNode(tools, handle_tool_errors=safe_tool_error, awrap_tool_call=observe_tool_call))
    graph.add_edge(START, "call_model")
    graph.add_conditional_edges("call_model", should_continue, {"tools": "tools", END: END})
    graph.add_edge("tools", "call_model")
    return graph.compile()
