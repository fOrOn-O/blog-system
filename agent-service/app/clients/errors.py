class BlogClientError(Exception):
    """Safe backend failure context; never retain an HTTP request or response."""

    def __init__(
        self, message: str, *, method: str, path: str, status_code: int | None = None
    ) -> None:
        super().__init__(message)
        self.method = method
        self.path = path
        self.status_code = status_code


class AuthenticationError(BlogClientError):
    """The user's credentials were rejected (401)."""


class AuthorizationError(BlogClientError):
    """The authenticated user is not allowed to perform the operation (403)."""


class ArticleNotFoundError(BlogClientError):
    """The article does not exist (404)."""


class VersionConflictError(BlogClientError):
    """409 conflict; Go currently uses this status for state conflicts too."""


class BlogRequestError(BlogClientError):
    """The backend rejected the request (other 4xx responses)."""


class BlogBackendError(BlogClientError):
    """Upstream server error or unexpected response contract."""


class BlogBackendUnavailableError(BlogClientError):
    """Timeout or transport failure; the outcome of a write may be unknown."""
