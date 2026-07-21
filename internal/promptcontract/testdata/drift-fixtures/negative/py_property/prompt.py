import openai
from models import PR
def build(pr: PR) -> str:
    return f"""You are a reviewer. PR is {pr.days_old} days old, created {pr.created}."""
