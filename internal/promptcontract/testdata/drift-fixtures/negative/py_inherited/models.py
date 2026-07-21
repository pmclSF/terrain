from pydantic import BaseModel
class Base(BaseModel):
    user_id: str
class Child(Base):
    name: str
