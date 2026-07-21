import openai
from pydantic import BaseModel

class SummaryState(BaseModel):
    research_topic: str

def run(state: SummaryState) -> str:
    state.session_id = _new_session()
    return f"""Research on {state.research_topic}, session {state.session_id}."""
