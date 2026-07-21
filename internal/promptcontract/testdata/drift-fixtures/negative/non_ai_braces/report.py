class Row:
    col_a: str
    col_b: str
def render(r):
    return f"""Report row {r.col_x} / {r.col_a}"""   # {r.col_x} mismatched, but NOT an AI surface
