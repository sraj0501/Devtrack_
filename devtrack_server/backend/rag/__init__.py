"""
RAG (Retrieval-Augmented Generation) layer for DevTrack personalization.

Indexes the user's communication samples into a local ChromaDB vector store
and retrieves semantically similar examples at inference time to inject as
few-shot style examples into every LLM prompt.

Components:
  embedder.py     — Ollama embedding API calls
  vector_store.py — ChromaDB persistent collection wrapper
  sample_indexer.py — high-level index/retrieve API (the main entry point)
"""
from .sample_indexer import SampleIndexer, get_indexer

__all__ = ["SampleIndexer", "get_indexer"]
