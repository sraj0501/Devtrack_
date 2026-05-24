"""
Shared text utilities for backend modules.

Provides common text processing functions used across multiple modules.
"""

import re
from typing import Optional


def clean_html_tags(text: Optional[str]) -> str:
    """
    Remove HTML tags and decode common HTML entities.

    Used by chat_analyzer, sentiment_analysis, and data_collectors.
    """
    if not text:
        return ""

    # Remove HTML tags
    text = re.sub(r'<[^>]+>', '', text)

    # Decode common HTML entities
    html_entities = {
        '&nbsp;': ' ',
        '&amp;': '&',
        '&lt;': '<',
        '&gt;': '>',
        '&quot;': '"',
        '&#39;': "'",
        '&apos;': "'"
    }

    for entity, replacement in html_entities.items():
        text = text.replace(entity, replacement)

    # Clean up extra whitespace
    return ' '.join(text.split())
