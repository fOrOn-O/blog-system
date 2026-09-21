class AgentExecutionError(Exception):
    """Safe execution failure without provider details or credentials."""

    def __init__(self, message="Agent execution failed", *, code="agent_execution_failed", status_code=502):
        super().__init__(message)
        self.code = code
        self.status_code = status_code


class WorkspaceRequiredError(Exception):
    pass


class CanonicalReadRequiredError(Exception):
    pass


class ProposalAlreadySubmittedError(Exception):
    pass


class MultipleProposalSubmissionsError(Exception):
    pass
