import openai
from dataclasses import dataclass
from PIL import Image as PILImage

@dataclass
class Image:
    image: "PILImage.Image"
    def __post_init__(self):
        self.image_str = _encode(self.image)
        self.image_format = "image/png"

def render(image: Image) -> str:
    return f"""data:{image.image_format};base64,{image.image_str}"""
