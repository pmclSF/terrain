import openai
from schema import Order

def summarize(order: Order) -> str:
    # BINDING: order: Order; {order.account_id} references a field Order lacks → DRIFT
    return f"""You are billing support. Summarize order {order.account_id}, total {order.total}."""
