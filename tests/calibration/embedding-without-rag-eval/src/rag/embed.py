"""RAG embedding pipeline. No retrieval eval scenario covers this file —
a future model swap will silently change retrieval quality."""

from langchain_openai import OpenAIEmbeddings


def make_embedder():
    return OpenAIEmbeddings(model="text-embedding-3-large")


def embed_documents(docs):
    return make_embedder().embed_documents(docs)
