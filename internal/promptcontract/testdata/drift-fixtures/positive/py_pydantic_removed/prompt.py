import openai
from models import UserProfile

def build(user: UserProfile) -> str:
    # BINDING: user: UserProfile; {user.user_id} references a field UserProfile no longer has → DRIFT
    return f"""You are a helpful assistant.
The user id is {user.user_id}; the name is {user.full_name}. Answer the question."""
