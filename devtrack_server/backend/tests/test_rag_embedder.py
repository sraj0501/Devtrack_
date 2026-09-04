"""Regression tests for bounded, batched first-run voice embedding."""

from unittest.mock import MagicMock, patch


def test_embed_batch_uses_one_ollama_request_for_all_inputs():
    from backend.rag.embedder import embed_batch

    vectors = [[0.1, 0.2], [0.3, 0.4]]
    response = MagicMock(status_code=200)
    response.json.return_value = {"embeddings": vectors}

    with (
        patch("backend.rag.embedder.model_available", return_value=True),
        patch("backend.rag.embedder.requests.post", return_value=response) as post,
    ):
        assert embed_batch(["first", "second"]) == vectors

    post.assert_called_once()
    assert post.call_args.kwargs["json"]["input"] == ["first", "second"]


def test_embed_batch_missing_model_fails_fast_without_posting():
    from backend.rag.embedder import embed_batch

    with (
        patch("backend.rag.embedder.model_available", return_value=False),
        patch("backend.rag.embedder.requests.post") as post,
    ):
        assert embed_batch(["first", "second"]) == [None, None]

    post.assert_not_called()


def test_embed_batch_preserves_alignment_on_bad_response():
    from backend.rag.embedder import embed_batch

    response = MagicMock(status_code=200)
    response.json.return_value = {"embeddings": [[0.1, 0.2]]}
    with (
        patch("backend.rag.embedder.model_available", return_value=True),
        patch("backend.rag.embedder.requests.post", return_value=response),
    ):
        assert embed_batch(["first", "second"]) == [None, None]
