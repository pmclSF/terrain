import openai
from models import Child
def build(c: Child) -> str:
    return f"""You are an assistant. User {c.user_id} named {c.name}. Respond."""
