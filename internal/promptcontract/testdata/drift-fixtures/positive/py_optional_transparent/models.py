from pydantic import BaseModel
class UserProfile(BaseModel):
    name: str
    email: str
