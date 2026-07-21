import openai
from requests import Response
def build(r: Response) -> str:
    return f"""You are support. The HTTP status was {r.status_code}."""
