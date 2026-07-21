import openai

TEMPLATE = "Hello {name}, your total is {total}."

def build(payload) -> str:
    return TEMPLATE.format(**payload)
