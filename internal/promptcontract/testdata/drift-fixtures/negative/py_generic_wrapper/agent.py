import openai
from dataclasses import dataclass

@dataclass
class MyDeps:
    user_name: str

# RunContext is a library generic (not defined in-repo); ctx.deps / ctx.run_step
# are RunContext attributes, not MyDeps fields.
def system_prompt(ctx: "RunContext[MyDeps]") -> str:
    return f"""You are helping {ctx.deps} on step {ctx.run_step}."""
