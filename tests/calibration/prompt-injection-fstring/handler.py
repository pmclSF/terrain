"""Chat handler that concatenates raw user input into a prompt.
Intentionally vulnerable for the calibration corpus."""

from openai import OpenAI

client = OpenAI()


def handle_chat(request):
    user_message = request.body["message"]
    system_prompt = f"You are a helpful assistant. The user said: {user_message}"
    response = client.chat.completions.create(
        model="gpt-4-0613",
        messages=[{"role": "system", "content": system_prompt}],
    )
    return response.choices[0].message.content
