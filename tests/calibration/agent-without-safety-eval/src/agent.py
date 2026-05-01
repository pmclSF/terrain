"""Routing agent — picks which downstream tool to invoke based on a
classifier. Has no safety eval coverage."""


def agent_router(query, available_tools):
    if "search" in query.lower():
        return available_tools.get("web_search")
    if "calculate" in query.lower():
        return available_tools.get("calculator")
    return None


def agent_planner(query, history):
    return {"steps": [{"tool": agent_router(query, history.tools), "input": query}]}
