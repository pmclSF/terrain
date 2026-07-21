from pydantic import BaseModel
class PR(BaseModel):
    created: str
    @property
    def days_old(self) -> int: return 0
