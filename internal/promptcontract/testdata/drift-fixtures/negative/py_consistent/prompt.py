import openai
from models import UserProfile
def build(user: UserProfile) -> str:
    return f"""You are an assistant. User {user.user_id} is {user.full_name}. Respond."""
