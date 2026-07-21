import openai
from typing import Optional
from models import UserProfile

def build(user: Optional[UserProfile]) -> str:
    return f"""You are an assistant for user {user.user_id}. Respond."""
