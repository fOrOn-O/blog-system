class RagError(RuntimeError):
    """对外只提供安全消息，不携带模型、网络或 SDK 的原始异常。"""


class RagConfigurationError(RagError):
    pass


class EmbeddingError(RagError):
    pass
