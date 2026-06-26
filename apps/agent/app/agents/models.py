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
