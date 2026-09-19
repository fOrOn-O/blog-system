"""显式预热、版本索引及 Agent 问答入口；JWT 使用隐藏输入，不进入命令行参数。"""

import argparse
import asyncio
from getpass import getpass

from app.agent.errors import AgentExecutionError
from app.agent.runner import AgentRunner
from app.clients.blog import BlogClient
from app.clients.errors import BlogClientError
from app.core.rag import close_rag_service, get_embedding_provider, get_rag_service
from app.rag.errors import RagError


async def run(args) -> None:
    if args.command == "warm-model":
        await asyncio.to_thread(get_embedding_provider().embed_query, "warm up")
        print("Embedding model ready")
        return
    token = getpass("JWT (hidden): ").strip()
    if not token or any(c.isspace() for c in token):
        raise ValueError("Invalid token")
    try:
        service = get_rag_service()
        async with BlogClient() as client:
            if args.command == "index":
                count = await service.index_article_version(
                    args.article_id, args.version_no, access_token=token, blog_client=client,
                )
                print(f"Indexed {count} chunks")
            else:
                runner = AgentRunner(rag_service=service)
                answer = await runner.run(
                    f"请检索文章 {args.article_id} 的版本 {args.version_no} 并回答：{args.question}",
                    access_token=token, blog_client=client,
                )
                print(answer.content)
    finally:
        await close_rag_service()


def main() -> None:
    parser = argparse.ArgumentParser(description="Explicit ArticleVersion RAG operations")
    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("warm-model")
    for name in ("index", "ask"):
        command = subparsers.add_parser(name)
        command.add_argument("--article-id", type=int, required=True)
        command.add_argument("--version-no", type=int, required=True)
        if name == "ask":
            command.add_argument("--question", required=True)
    args = parser.parse_args()
    try:
        asyncio.run(run(args))
    except (BlogClientError, RagError, AgentExecutionError, ValueError) as error:
        # 只输出异常类别，不把配置校验值、后端消息或凭据写入终端。
        parser.exit(1, f"RAG operation failed ({type(error).__name__}); check configuration, ownership and index.\n")


if __name__ == "__main__":
    main()
