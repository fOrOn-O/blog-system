class AgentExecutionError(Exception):
    """Safe execution failure without provider details or credentials."""


class WorkspaceRequiredError(Exception):
    pass


class CanonicalReadRequiredError(Exception):
    pass


class ProposalAlreadySubmittedError(Exception):
    pass


class MultipleProposalSubmissionsError(Exception):
    pass
