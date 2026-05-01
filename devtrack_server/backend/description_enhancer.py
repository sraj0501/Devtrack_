"""
Description Enhancer using Ollama

This module takes raw user input describing their work and enhances it 
to be more professional, clear, and suitable for task tracking systems.
"""

import logging
from typing import Dict, Optional, List
from dataclasses import dataclass

logger = logging.getLogger(__name__)


@dataclass
class EnhancedDescription:
    """Represents an enhanced description result"""
    original: str
    enhanced: str
    summary: str  # One-line summary
    category: str  # feature, bugfix, refactor, docs, etc.
    keywords: List[str]
    confidence: float
    
    def to_dict(self) -> Dict:
        return {
            "original": self.original,
            "enhanced": self.enhanced,
            "summary": self.summary,
            "category": self.category,
            "keywords": self.keywords,
            "confidence": self.confidence
        }


class DescriptionEnhancer:
    """Enhances task descriptions using the configured LLM provider."""

    CATEGORIES = [
        "feature",      # New functionality
        "bugfix",       # Bug fixes
        "refactor",     # Code refactoring
        "docs",         # Documentation
        "test",         # Testing
        "config",       # Configuration changes
        "review",       # Code review
        "meeting",      # Meetings and discussions
        "research",     # Research and investigation
        "support",      # Support and troubleshooting
        "devops",       # CI/CD, deployment
        "other"         # General work
    ]
    
    def __init__(self, provider=None):
        """Initialize the description enhancer.

        Args:
            provider: Optional LLM provider instance (injected for testing).
                      Defaults to the global provider chain from get_provider().
        """
        self._provider = provider  # None = lazy init on first use

    def _get_provider(self):
        if self._provider is None:
            from backend.llm import get_provider
            self._provider = get_provider()
        return self._provider

    def is_available(self) -> bool:
        """Check if the LLM provider is available."""
        return self._get_provider().primary.is_available()
    
    def enhance(self, raw_input: str, context: Optional[Dict] = None) -> EnhancedDescription:
        """Enhance a raw user description.

        Args:
            raw_input: The raw user input describing their work
            context: Optional context (project name, recent tickets, etc.)

        Returns:
            EnhancedDescription object with enhanced text
        """
        if not raw_input or not raw_input.strip():
            return self._create_empty_result(raw_input)

        # Try LLM enhancement
        try:
            return self._enhance_with_llm(raw_input, context)
        except Exception as e:
            logger.warning(f"LLM enhancement failed: {e}")

        # Fallback to basic enhancement
        return self._basic_enhance(raw_input, context)
    
    def _enhance_with_llm(self, raw_input: str, context: Optional[Dict] = None) -> EnhancedDescription:
        """Use the configured LLM provider to enhance the description."""

        # Build context string
        context_str = ""
        if context:
            if context.get("project"):
                context_str += f"Project: {context['project']}\n"
            if context.get("ticket_id"):
                context_str += f"Ticket: {context['ticket_id']}\n"
            if context.get("recent_work"):
                context_str += f"Recent work: {context['recent_work']}\n"

        prompt = f"""You are a technical writer helping a developer describe their work for task tracking.

Take this raw description of work done and enhance it to be:
1. Professional and clear
2. Action-oriented (starts with a verb)
3. Specific about what was accomplished
4. Concise but complete (1-3 sentences)

{f'Context:{chr(10)}{context_str}' if context_str else ''}
Raw input: "{raw_input}"

Respond in this exact format (each field on its own line):
ENHANCED: <enhanced description, 1-3 clear sentences>
SUMMARY: <one-line summary under 80 characters>
CATEGORY: <one of: {', '.join(self.CATEGORIES)}>
KEYWORDS: <comma-separated keywords>

Examples:
Raw: "fixed that bug in login"
ENHANCED: Fixed authentication bug that was preventing users from logging in. Updated session handling logic and added proper error messages.
SUMMARY: Fixed login authentication bug
CATEGORY: bugfix
KEYWORDS: authentication, login, bugfix, session

Raw: "working on the new api stuff"
ENHANCED: Implementing new REST API endpoints for user management. Added CRUD operations and integrated with the existing database layer.
SUMMARY: Implementing user management API endpoints
CATEGORY: feature
KEYWORDS: API, REST, endpoints, user-management

Now enhance this input:"""

        from backend.llm.base import LLMOptions
        from backend.config import http_timeout, description_llm_temperature, description_llm_max_tokens
        response_text = self._get_provider().generate(
            prompt=prompt,
            options=LLMOptions(
                temperature=description_llm_temperature(),
                max_tokens=description_llm_max_tokens(),
            ),
            timeout=http_timeout(),
        )

        if not response_text:
            return self._basic_enhance(raw_input, context)

        return self._parse_ollama_response(raw_input, response_text)
    
    def _parse_ollama_response(self, raw_input: str, response_text: str) -> EnhancedDescription:
        """Parse the structured response from Ollama"""
        enhanced = ""
        summary = ""
        category = "other"
        keywords = []
        
        for line in response_text.split('\n'):
            line = line.strip()
            if line.startswith("ENHANCED:"):
                enhanced = line.replace("ENHANCED:", "").strip()
            elif line.startswith("SUMMARY:"):
                summary = line.replace("SUMMARY:", "").strip()
            elif line.startswith("CATEGORY:"):
                cat = line.replace("CATEGORY:", "").strip().lower()
                if cat in self.CATEGORIES:
                    category = cat
            elif line.startswith("KEYWORDS:"):
                kw_str = line.replace("KEYWORDS:", "").strip()
                keywords = [k.strip() for k in kw_str.split(",") if k.strip()]
        
        # Fallback if parsing failed
        if not enhanced:
            # Try to use the first non-empty line
            lines = [line.strip() for line in response_text.split('\n') if line.strip()]
            if lines:
                enhanced = lines[0]
            else:
                enhanced = raw_input
        
        if not summary:
            summary = enhanced[:80] if enhanced else raw_input[:80]
        
        # Calculate confidence based on parsing success
        confidence = 0.9 if enhanced and summary and category != "other" else 0.7
        
        return EnhancedDescription(
            original=raw_input,
            enhanced=enhanced,
            summary=summary,
            category=category,
            keywords=keywords,
            confidence=confidence
        )
    
    def _basic_enhance(self, raw_input: str, context: Optional[Dict] = None) -> EnhancedDescription:
        """Basic enhancement when Ollama is not available"""
        # Clean up the input
        text = raw_input.strip()
        
        # Capitalize first letter
        if text and text[0].islower():
            text = text[0].upper() + text[1:]
        
        # Ensure it ends with proper punctuation
        if text and text[-1] not in '.!?':
            text += '.'
        
        # Detect category based on keywords
        category = self._detect_category(raw_input)
        
        # Extract keywords
        keywords = self._extract_keywords(raw_input)
        
        # Create summary (first sentence or truncated)
        summary = text.split('.')[0]
        if len(summary) > 80:
            summary = summary[:77] + "..."
        
        return EnhancedDescription(
            original=raw_input,
            enhanced=text,
            summary=summary,
            category=category,
            keywords=keywords,
            confidence=0.5  # Lower confidence for basic enhancement
        )
    
    def _detect_category(self, text: str) -> str:
        """Detect category from text keywords"""
        text_lower = text.lower()
        
        category_keywords = {
            "bugfix": ["fix", "bug", "error", "issue", "problem", "crash", "broken"],
            "feature": ["add", "new", "implement", "create", "feature", "functionality"],
            "refactor": ["refactor", "clean", "reorganize", "restructure", "improve"],
            "docs": ["doc", "readme", "comment", "documentation", "wiki"],
            "test": ["test", "spec", "coverage", "unit test", "integration"],
            "config": ["config", "setup", "env", "setting", "configure"],
            "review": ["review", "feedback", "pr", "pull request", "code review"],
            "meeting": ["meeting", "call", "discussion", "standup", "sync"],
            "research": ["research", "investigate", "explore", "study", "analyze"],
            "support": ["support", "help", "assist", "troubleshoot", "debug"],
            "devops": ["deploy", "ci", "cd", "pipeline", "docker", "kubernetes"],
        }
        
        for category, keywords in category_keywords.items():
            for keyword in keywords:
                if keyword in text_lower:
                    return category
        
        return "other"
    
    def _extract_keywords(self, text: str) -> List[str]:
        """Extract keywords from text"""
        import re
        
        # Common words to ignore
        stopwords = {
            "the", "a", "an", "is", "are", "was", "were", "be", "been",
            "being", "have", "has", "had", "do", "does", "did", "will",
            "would", "could", "should", "may", "might", "must", "shall",
            "can", "need", "for", "from", "with", "about", "into", "through",
            "during", "before", "after", "above", "below", "to", "of", "in",
            "on", "at", "by", "as", "and", "or", "but", "if", "then", "else",
            "when", "where", "why", "how", "all", "each", "every", "both",
            "few", "more", "most", "other", "some", "such", "no", "nor",
            "not", "only", "own", "same", "so", "than", "too", "very", "just",
            "also", "now", "here", "there", "this", "that", "these", "those",
            "i", "me", "my", "we", "our", "you", "your", "it", "its"
        }
        
        # Extract words
        words = re.findall(r'\b[a-zA-Z]{3,}\b', text.lower())
        
        # Filter and dedupe
        keywords = []
        seen = set()
        for word in words:
            if word not in stopwords and word not in seen:
                keywords.append(word)
                seen.add(word)
                if len(keywords) >= 5:
                    break
        
        return keywords
    
    def _create_empty_result(self, raw_input: str) -> EnhancedDescription:
        """Create result for empty input"""
        return EnhancedDescription(
            original=raw_input or "",
            enhanced="No work description provided.",
            summary="No description",
            category="other",
            keywords=[],
            confidence=0.0
        )


# Quick helper function
def enhance_description(text: str, context: Optional[Dict] = None) -> EnhancedDescription:
    """
    Quick helper to enhance a description
    
    Args:
        text: Raw description text
        context: Optional context dictionary
        
    Returns:
        EnhancedDescription object
    """
    enhancer = DescriptionEnhancer()
    return enhancer.enhance(text, context)


# Example usage
if __name__ == "__main__":
    examples = [
        "fixed that bug in the login page",
        "working on api stuff for new feature",
        "had meeting with team about sprint planning",
        "reviewed johns pr for the payment module",
        "debugging the memory leak issue in production",
    ]
    
    enhancer = DescriptionEnhancer()
    
    print("Description Enhancer Examples")
    print("=" * 60)
    print(f"LLM available: {enhancer.is_available()}")
    print()
    
    for text in examples:
        print(f"Input: {text}")
        print("-" * 40)
        result = enhancer.enhance(text)
        print(f"Enhanced: {result.enhanced}")
        print(f"Summary:  {result.summary}")
        print(f"Category: {result.category}")
        print(f"Keywords: {', '.join(result.keywords)}")
        print(f"Confidence: {result.confidence:.2f}")
        print()
