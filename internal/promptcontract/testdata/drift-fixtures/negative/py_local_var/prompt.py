import openai
def build(question: str) -> str:
    tmp = question.strip()
    return f"""You are a helpful assistant. Answer the following: {tmp}"""   # {tmp} is a local
