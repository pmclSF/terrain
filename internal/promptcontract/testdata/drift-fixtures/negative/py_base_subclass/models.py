import openai
from pydantic import BaseModel

class WorkflowNode(BaseModel):
    node_id: str

class AgentNode(WorkflowNode):
    agent_name: str

def describe(node: WorkflowNode) -> str:
    return f"""Calling agent {node.agent_name} for node {node.node_id}."""
