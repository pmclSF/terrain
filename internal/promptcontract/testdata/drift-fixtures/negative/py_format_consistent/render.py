import openai
from models import Order

TEMPLATE = "Order {order_id} for {customer}."

def build(order: Order) -> str:
    return TEMPLATE.format(**order.dict())
