import openai
from models import Order

TEMPLATE = """You are billing support.
Order {order_id} for {customer} on account {account_id}. Respond."""

def build(order: Order) -> str:
    return TEMPLATE.format(**order.model_dump())
