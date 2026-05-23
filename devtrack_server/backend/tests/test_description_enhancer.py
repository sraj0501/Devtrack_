"""
Tests for backend.description_enhancer module.
"""
import pytest


def test_basic_enhance_when_ollama_unavailable():
    """Test _basic_enhance works when Ollama is not available."""
    from backend.description_enhancer import DescriptionEnhancer, EnhancedDescription

    enhancer = DescriptionEnhancer()
    result = enhancer._basic_enhance("fixed that bug in login")
    assert isinstance(result, EnhancedDescription)
    assert result.original == "fixed that bug in login"
    assert result.enhanced is not None
    assert len(result.enhanced) > 0
    assert result.category == "bugfix"
    assert result.confidence == 0.5


def test_detect_category_bugfix():
    """Test category detection for bugfix keywords."""
    from backend.description_enhancer import DescriptionEnhancer

    enhancer = DescriptionEnhancer()
    assert enhancer._detect_category("fixed the bug") == "bugfix"
    assert enhancer._detect_category("resolved the error") == "bugfix"


def test_detect_category_feature():
    """Test category detection for feature keywords."""
    from backend.description_enhancer import DescriptionEnhancer

    enhancer = DescriptionEnhancer()
    assert enhancer._detect_category("implemented new feature") == "feature"
    assert enhancer._detect_category("added new functionality") == "feature"


def test_detect_category_other():
    """Test category detection falls back to other."""
    from backend.description_enhancer import DescriptionEnhancer

    enhancer = DescriptionEnhancer()
    assert enhancer._detect_category("something random") == "other"


def test_enhance_empty_input():
    """Test enhance returns empty result for empty input."""
    from backend.description_enhancer import DescriptionEnhancer, EnhancedDescription

    enhancer = DescriptionEnhancer()
    result = enhancer.enhance("")
    assert isinstance(result, EnhancedDescription)
    assert result.enhanced == "No work description provided."
    assert result.confidence == 0.0


def test_enhance_with_ollama_optional():
    """Test enhance works (uses basic fallback) when Ollama unavailable."""
    from backend.description_enhancer import DescriptionEnhancer

    enhancer = DescriptionEnhancer()
    # Will use basic enhancement if Ollama not available
    result = enhancer.enhance("working on api stuff")
    assert result is not None
    assert result.enhanced is not None
    assert len(result.enhanced) > 0
