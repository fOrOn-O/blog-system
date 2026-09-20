"""显式全站修复或单篇同步；不保存 JWT，不自动重试。"""
import argparse
import asyncio
from getpass import getpass

from app.clients.blog import BlogClient
from app.core.rag import get_published_service, close_published_service


async def run(args):
    token = getpass("JWT (hidden): ").strip()
    try:
        async with BlogClient() as blog:
            await blog.get_published_knowledge(access_token=token)
            service = get_published_service()
            if args.command == "reconcile":
                count = await service.reconcile(access_token=token, blog_client=blog)
                print(f"Reconciled {count} articles")
            else:
                count = await service.sync_article(args.article_id, access_token=token, blog_client=blog)
                print(f"Indexed {count} chunks")
    finally:
        await close_published_service()


def main():
    parser = argparse.ArgumentParser(description="Published knowledge sync/reconcile")
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("reconcile")
    commands.add_parser("sync").add_argument("--article-id", type=int, required=True)
    try:
        asyncio.run(run(parser.parse_args()))
    except Exception as error:
        parser.exit(1, f"Knowledge operation failed ({type(error).__name__}); retry explicitly after checking configuration.\n")


if __name__ == "__main__":
    main()
