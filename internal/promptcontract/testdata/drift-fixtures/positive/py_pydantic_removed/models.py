from pydantic import BaseModel

class UserProfile(BaseModel):
    full_name: str
    email: str
    account_id: str        # `user_id` was renamed to `account_id`; the prompt wasn't updated
