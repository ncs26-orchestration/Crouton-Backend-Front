"""Typed output models for the intake and department agents."""

from pydantic import BaseModel, Field


class PlanNode(BaseModel):
    """A single stage in the planned workflow."""

    key: str = Field(description="Unique key for this stage, e.g. 'finance_review'")
    name: str = Field(description="Human-readable stage name")
    agent_type: str = Field(description="The agent type that handles this stage")
    department: str = Field(description="Department that owns this stage")


class PlanEdge(BaseModel):
    """An edge connecting two workflow stages."""

    from_: str = Field(alias="from", description="Source stage key")
    to: str = Field(description="Target stage key")
    type: str = Field(default="sequence", description="Edge type")

    model_config = {"populate_by_name": True}


class Plan(BaseModel):
    """The intake agent's output: a workflow graph for a business request."""

    nodes: list[PlanNode] = Field(description="Workflow stages")
    edges: list[PlanEdge] = Field(description="Connections between stages")


class Flag(BaseModel):
    """A risk or note a department agent surfaces while reviewing a request."""

    severity: str = Field(description="One of: info, warning, critical")
    message: str = Field(description="Plain-language description of the flag")


class TaskItem(BaseModel):
    """A unit of work a department agent performed on a node."""

    title: str = Field(description="What the agent did, e.g. 'Assess budget feasibility'")
    status: str = Field(default="completed", description="pending, in_progress, or completed")


class DependencyDecl(BaseModel):
    """A cross-department dependency an agent declares when it is blocked.

    Populated by the ``raise_dependency`` tool (F5); ``None`` here means the
    department could complete without waiting on another.
    """

    on_department: str = Field(description="Department this stage is waiting on")
    reason: str = Field(description="The agent's own reason for the dependency")


class Decision(BaseModel):
    """A department agent's output for one workflow node."""

    summary: str = Field(description="Short summary of the department's assessment")
    flags: list[Flag] = Field(default_factory=list, description="Risks or notes surfaced")
    tasks: list[TaskItem] = Field(default_factory=list, description="Work performed on this node")
    status_text: str = Field(description="Plain-language status for the UI")
    blocked_on: DependencyDecl | None = Field(
        default=None, description="Set when the agent is blocked on another department (F5)"
    )
