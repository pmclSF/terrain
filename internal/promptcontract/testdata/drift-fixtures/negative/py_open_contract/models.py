import openai
from dataclasses import dataclass

@dataclass(init=False)
class Req:
    prompt: str
    def __init__(self, **kwargs):
        for name, value in kwargs.items():
            setattr(self, name, value)
    def __getattr__(self, name):
        return getattr(self.sampling_params, name)

def build(batch: Req) -> str:
    return f"""Running {batch.num_inference_steps} steps at scale {batch.guidance_scale}."""
