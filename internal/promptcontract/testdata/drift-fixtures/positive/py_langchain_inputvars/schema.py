from pydantic import BaseModel

class Order(BaseModel):
    order_id: str          # not account_id
    total: float
