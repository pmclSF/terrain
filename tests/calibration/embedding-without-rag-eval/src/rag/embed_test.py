"""A test that exercises embed_documents but is not a retrieval eval —
it doesn't mention retrieval/RAG/vector/knn anywhere, so it should not
suppress the aiEmbeddingModelChange signal."""

from .embed import embed_documents


def test_embed_documents_returns_list_per_doc():
    out = embed_documents(["a", "b"])
    assert len(out) == 2
