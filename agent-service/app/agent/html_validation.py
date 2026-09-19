"""提案的最小 HTML 结构校验；不做分块、补全、改写或 HTML 清洗。"""

from html.parser import HTMLParser


class _ProposalHTMLParser(HTMLParser):
    VOID_TAGS = {"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr"}

    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.stack: list[str] = []
        self.has_element = False
        self.has_content = False

    def handle_starttag(self, tag, attrs):
        self.has_element = True
        if tag in {"script", "style", "iframe", "object", "embed", "form", "input", "base", "link", "meta"}:
            raise ValueError("Active HTML is not supported in proposals")
        if any(name.lower().startswith("on") for name, _ in attrs):
            raise ValueError("HTML event attributes are not supported in proposals")
        if tag not in self.VOID_TAGS:
            self.stack.append(tag)
        if tag == "img" and (dict(attrs).get("src") or "").strip():
            self.has_content = True

    def handle_startendtag(self, tag, attrs):
        self.handle_starttag(tag, attrs)
        if tag not in self.VOID_TAGS:
            self.handle_endtag(tag)

    def handle_endtag(self, tag):
        if not self.stack or self.stack[-1] != tag:
            raise ValueError("HTML tags must be balanced")
        self.stack.pop()

    def handle_data(self, data):
        if data.strip():
            if not self.stack:
                raise ValueError("Proposal must contain HTML, not plain text or a Markdown fence")
            self.has_content = True


def validate_proposed_html(content: str) -> str:
    if not content.strip():
        raise ValueError("Proposed HTML must not be empty")
    parser = _ProposalHTMLParser()
    parser.feed(content)
    parser.close()
    if parser.stack or not parser.has_element or not parser.has_content:
        raise ValueError("Proposal must contain complete nonempty HTML")
    return content
