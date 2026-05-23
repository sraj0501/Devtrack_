"""
Task Matching & Fuzzy Logic System

This module implements semantic similarity matching and fuzzy string matching
to automatically match user updates to existing tasks in Azure DevOps, GitHub, or Jira.
"""

import asyncio
import re
from typing import List, Optional, Tuple
from dataclasses import dataclass
from fuzzywuzzy import fuzz
import logging

# Configure logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


@dataclass
class Task:
    """Represents a task from project management system"""
    id: str
    title: str
    description: str
    status: str
    project: str
    assignee: Optional[str] = None
    tags: List[str] = None
    source: str = "unknown"  # 'azure', 'github', 'jira'
    
    def __post_init__(self):
        if self.tags is None:
            self.tags = []


@dataclass
class MatchResult:
    """Result of task matching"""
    task: Task
    confidence: float
    match_type: str  # 'exact', 'fuzzy', 'semantic', 'partial'
    match_field: str  # 'id', 'title', 'description'
    reason: str


class TaskMatcher:
    """Matches user updates to existing tasks"""
    
    def __init__(self, use_semantic: bool = True):
        """
        Initialize task matcher
        
        Args:
            use_semantic: Whether to use semantic similarity (requires sentence-transformers)
        """
        self.use_semantic = use_semantic
        self.semantic_model = None
        
        if use_semantic:
            try:
                from sentence_transformers import SentenceTransformer
                model_name = 'all-MiniLM-L6-v2'
                try:
                    from backend.config import semantic_model_name
                    model_name = semantic_model_name()
                except ImportError:
                    pass
                self.semantic_model = SentenceTransformer(model_name)
                logger.info("Loaded semantic similarity model")
            except ImportError:
                logger.warning("sentence-transformers not installed, falling back to fuzzy matching")
                self.use_semantic = False
    
    def match_task(
        self,
        user_input: str,
        available_tasks: List[Task],
        threshold: float = 0.6
    ) -> Optional[MatchResult]:
        """
        Find best matching task for user input
        
        Args:
            user_input: User's description of work
            available_tasks: List of tasks to match against
            threshold: Minimum confidence threshold (0.0 to 1.0)
            
        Returns:
            MatchResult if match found above threshold, None otherwise
        """
        if not available_tasks:
            return None
        
        # Try exact ID match first
        exact_match = self._try_exact_id_match(user_input, available_tasks)
        if exact_match:
            return exact_match
        
        # Try fuzzy title matching
        fuzzy_match = self._try_fuzzy_match(user_input, available_tasks, threshold)
        if fuzzy_match and fuzzy_match.confidence >= threshold:
            return fuzzy_match
        
        # Try semantic matching if available
        if self.use_semantic and self.semantic_model:
            semantic_match = self._try_semantic_match(user_input, available_tasks, threshold)
            if semantic_match and semantic_match.confidence >= threshold:
                return semantic_match
        
        # Return best fuzzy match even if below threshold (for user confirmation)
        return fuzzy_match
    
    def match_multiple(
        self,
        user_input: str,
        available_tasks: List[Task],
        top_n: int = 3,
        threshold: float = 0.5
    ) -> List[MatchResult]:
        """
        Find multiple possible task matches
        
        Args:
            user_input: User's description of work
            available_tasks: List of tasks to match against
            top_n: Number of top matches to return
            threshold: Minimum confidence threshold
            
        Returns:
            List of MatchResults sorted by confidence
        """
        matches = []
        
        # Try exact ID match
        exact_match = self._try_exact_id_match(user_input, available_tasks)
        if exact_match:
            return [exact_match]  # Exact match, no need for others
        
        # Get fuzzy matches
        fuzzy_matches = self._get_all_fuzzy_matches(user_input, available_tasks)
        matches.extend(fuzzy_matches)
        
        # Get semantic matches if available
        if self.use_semantic and self.semantic_model:
            semantic_matches = self._get_all_semantic_matches(user_input, available_tasks)
            matches.extend(semantic_matches)
        
        # Deduplicate and sort by confidence
        seen_ids = set()
        unique_matches = []
        
        for match in sorted(matches, key=lambda m: m.confidence, reverse=True):
            if match.task.id not in seen_ids:
                if match.confidence >= threshold:
                    unique_matches.append(match)
                    seen_ids.add(match.task.id)
            
            if len(unique_matches) >= top_n:
                break
        
        return unique_matches
    
    def _try_exact_id_match(self, user_input: str, tasks: List[Task]) -> Optional[MatchResult]:
        """Try to find exact task ID in user input"""
        # Extract possible task IDs from user input
        # Patterns: #123, PROJ-456, AB-789, etc.
        patterns = [
            r'#(\d+)',  # #123
            r'([A-Z]+-\d+)',  # PROJ-123
            r'([A-Z]{2,}-\d+)',  # AB-123
        ]
        
        for pattern in patterns:
            matches = re.findall(pattern, user_input, re.IGNORECASE)
            for match_id in matches:
                # Try to find this ID in tasks
                for task in tasks:
                    if match_id.lower() in task.id.lower():
                        return MatchResult(
                            task=task,
                            confidence=1.0,
                            match_type='exact',
                            match_field='id',
                            reason=f"Exact ID match: {match_id}"
                        )
        
        return None
    
    def _try_fuzzy_match(
        self,
        user_input: str,
        tasks: List[Task],
        threshold: float
    ) -> Optional[MatchResult]:
        """Use fuzzy string matching on task titles"""
        best_match = None
        best_score = 0
        
        for task in tasks:
            # Try different fuzzy matching algorithms
            ratio_score = fuzz.ratio(user_input.lower(), task.title.lower()) / 100.0
            partial_score = fuzz.partial_ratio(user_input.lower(), task.title.lower()) / 100.0
            token_score = fuzz.token_sort_ratio(user_input.lower(), task.title.lower()) / 100.0
            
            # Use the best score
            score = max(ratio_score, partial_score, token_score)
            
            if score > best_score:
                best_score = score
                match_type = 'fuzzy'
                if score == partial_score and score > ratio_score:
                    match_type = 'partial'
                
                best_match = MatchResult(
                    task=task,
                    confidence=score,
                    match_type=match_type,
                    match_field='title',
                    reason=f"Fuzzy match on title (score: {score:.2f})"
                )
        
        return best_match if best_score >= threshold else None
    
    def _get_all_fuzzy_matches(self, user_input: str, tasks: List[Task]) -> List[MatchResult]:
        """Get all fuzzy matches above minimum threshold"""
        matches = []
        
        for task in tasks:
            ratio_score = fuzz.ratio(user_input.lower(), task.title.lower()) / 100.0
            partial_score = fuzz.partial_ratio(user_input.lower(), task.title.lower()) / 100.0
            token_score = fuzz.token_sort_ratio(user_input.lower(), task.title.lower()) / 100.0
            
            score = max(ratio_score, partial_score, token_score)
            
            if score >= 0.4:  # Minimum threshold for consideration
                match_type = 'fuzzy'
                if score == partial_score and score > ratio_score:
                    match_type = 'partial'
                
                matches.append(MatchResult(
                    task=task,
                    confidence=score,
                    match_type=match_type,
                    match_field='title',
                    reason=f"Fuzzy match (score: {score:.2f})"
                ))
        
        return matches
    
    def _try_semantic_match(
        self,
        user_input: str,
        tasks: List[Task],
        threshold: float
    ) -> Optional[MatchResult]:
        """Use semantic similarity (sentence transformers)"""
        if not self.semantic_model:
            return None
        
        import numpy as np
        from sklearn.metrics.pairwise import cosine_similarity
        
        # Encode user input
        user_embedding = self.semantic_model.encode([user_input])
        
        best_match = None
        best_score = 0
        
        for task in tasks:
            # Combine title and description for better matching
            task_text = f"{task.title} {task.description}"
            task_embedding = self.semantic_model.encode([task_text])
            
            # Calculate cosine similarity
            similarity = cosine_similarity(user_embedding, task_embedding)[0][0]
            
            if similarity > best_score:
                best_score = similarity
                best_match = MatchResult(
                    task=task,
                    confidence=float(similarity),
                    match_type='semantic',
                    match_field='title+description',
                    reason=f"Semantic similarity (score: {similarity:.2f})"
                )
        
        return best_match if best_score >= threshold else None
    
    def _get_all_semantic_matches(self, user_input: str, tasks: List[Task]) -> List[MatchResult]:
        """Get all semantic matches above minimum threshold"""
        if not self.semantic_model:
            return []
        
        import numpy as np
        from sklearn.metrics.pairwise import cosine_similarity
        
        matches = []
        user_embedding = self.semantic_model.encode([user_input])
        
        for task in tasks:
            task_text = f"{task.title} {task.description}"
            task_embedding = self.semantic_model.encode([task_text])
            
            similarity = cosine_similarity(user_embedding, task_embedding)[0][0]
            
            if similarity >= 0.3:  # Minimum threshold
                matches.append(MatchResult(
                    task=task,
                    confidence=float(similarity),
                    match_type='semantic',
                    match_field='title+description',
                    reason=f"Semantic similarity (score: {similarity:.2f})"
                ))
        
        return matches
    
    def disambiguate(
        self,
        matches: List[MatchResult],
        user_input: str
    ) -> Tuple[Optional[MatchResult], str]:
        """
        Help user disambiguate between multiple matches
        
        Args:
            matches: List of possible matches
            user_input: Original user input
            
        Returns:
            Tuple of (selected match or None, prompt text for user)
        """
        if not matches:
            return None, "No matching tasks found. Please provide more details or task ID."
        
        if len(matches) == 1:
            match = matches[0]
            if match.confidence >= 0.8:
                return match, f"Matched to: {match.task.title} (confidence: {match.confidence:.0%})"
            else:
                prompt = f"Did you mean: {match.task.title}? (confidence: {match.confidence:.0%})\n"
                prompt += "Reply 'yes' to confirm or provide more details."
                return match, prompt
        
        # Multiple matches - ask user to choose
        prompt = "Multiple possible matches found:\n\n"
        for i, match in enumerate(matches, 1):
            prompt += f"{i}. [{match.task.id}] {match.task.title}\n"
            prompt += f"   Confidence: {match.confidence:.0%} ({match.match_type} match)\n"
            if match.task.status:
                prompt += f"   Status: {match.task.status}\n"
            prompt += "\n"
        
        prompt += "Please reply with the number of the correct task, or provide more details."
        
        return None, prompt


class TaskRepository:
    """Repository for fetching tasks from various sources"""
    
    def __init__(self):
        self.azure_client = None
        self.github_client = None
        self.jira_client = None
    
    def initialize_azure(self, azure_client):
        """Initialize Azure DevOps client"""
        self.azure_client = azure_client
    
    def initialize_github(self, github_client):
        """Initialize GitHub client"""
        self.github_client = github_client
    
    def initialize_jira(self, jira_client):
        """Initialize Jira client"""
        self.jira_client = jira_client
    
    def get_my_tasks(self, source: str = 'all') -> List[Task]:
        """
        Get tasks assigned to current user
        
        Args:
            source: 'azure', 'github', 'jira', or 'all'
            
        Returns:
            List of tasks
        """
        tasks = []
        
        if source in ['azure', 'all'] and self.azure_client:
            tasks.extend(self._get_azure_tasks())
        
        if source in ['github', 'all'] and self.github_client:
            tasks.extend(self._get_github_tasks())
        
        if source in ['jira', 'all'] and self.jira_client:
            tasks.extend(self._get_jira_tasks())
        
        return tasks
    
    def _get_azure_tasks(self) -> List[Task]:
        """Fetch tasks from Azure DevOps (sync wrapper around async)."""
        if not self.azure_client:
            return []
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            loop = None

        if loop and loop.is_running():
            # Already inside an event loop — caller must use get_azure_tasks_async
            logger.warning(
                "_get_azure_tasks called inside a running event loop; "
                "use get_azure_tasks_async instead"
            )
            return []

        return asyncio.run(self.get_azure_tasks_async())

    async def get_azure_tasks_async(self) -> List[Task]:
        """Fetch tasks from Azure DevOps asynchronously."""
        if not self.azure_client:
            return []
        logger.info("Fetching tasks from Azure DevOps...")
        try:
            from backend import config
            states = config.get_azure_sync_states()
            work_items = await self.azure_client.get_my_work_items(states=states)
            return [
                Task(
                    id=str(wi.id),
                    title=wi.title,
                    description=wi.description or "",
                    status=wi.state,
                    project=wi.area_path.split("\\")[0] if wi.area_path else "",
                    assignee=wi.assigned_to or "",
                    tags=wi.tags,
                    source="azure",
                )
                for wi in work_items
            ]
        except Exception as e:
            logger.warning(f"Failed to fetch Azure DevOps tasks: {e}")
            return []

    async def get_my_tasks_async(self, source: str = "all") -> List[Task]:
        """Async variant of get_my_tasks — required when Azure client is in use.

        Jira and GitHub fetching is still synchronous (wrapped), but Azure
        tasks are awaited properly.
        """
        tasks: List[Task] = []

        if source in ("azure", "all") and self.azure_client:
            tasks.extend(await self.get_azure_tasks_async())

        if source in ("github", "all") and self.github_client:
            tasks.extend(self._get_github_tasks())

        if source in ("jira", "all") and self.jira_client:
            tasks.extend(self._get_jira_tasks())

        return tasks
    
    def _get_github_tasks(self) -> List[Task]:
        """Fetch tasks from GitHub"""
        # Placeholder - would use actual GitHub API
        logger.info("Fetching tasks from GitHub...")
        return []
    
    def _get_jira_tasks(self) -> List[Task]:
        """Fetch tasks from Jira using the configured JiraClient."""
        if not self.jira_client:
            return []
        logger.info("Fetching tasks from Jira...")
        try:
            issues = self.jira_client.get_my_issues()
            return [
                Task(
                    id=issue.key,
                    title=issue.summary,
                    description=issue.description,
                    status=issue.status,
                    project=issue.key.split("-")[0] if "-" in issue.key else issue.key,
                    assignee=issue.assignee or "",
                    tags=issue.labels,
                    source="jira",
                )
                for issue in issues
            ]
        except Exception as e:
            logger.warning(f"Failed to fetch Jira tasks: {e}")
            return []


# CLI interface
def main():
    """CLI for task matching"""
    import sys
    
    if len(sys.argv) < 2:
        print("Usage: python task_matcher.py <command> [options]")
        print()
        print("Commands:")
        print("  match <text>           Find matching task for text")
        print("  test                   Run test with sample data")
        print()
        return
    
    command = sys.argv[1]
    
    if command == 'test':
        # Create sample tasks
        tasks = [
            Task(
                id="PROJ-123",
                title="Fix login authentication bug",
                description="Users unable to login with SSO",
                status="In Progress",
                project="Authentication"
            ),
            Task(
                id="PROJ-124",
                title="Add OAuth2 support",
                description="Implement OAuth2 authentication flow",
                status="New",
                project="Authentication"
            ),
            Task(
                id="PROJ-125",
                title="Update user profile page",
                description="Redesign user settings interface",
                status="In Progress",
                project="UI/UX"
            ),
        ]
        
        # Test cases
        test_inputs = [
            "Fixed the login bug PROJ-123",  # Exact ID
            "working on authentication issues",  # Fuzzy match
            "redesigning the profile settings",  # Semantic match
        ]
        
        matcher = TaskMatcher(use_semantic=True)
        
        print("=" * 70)
        print("TASK MATCHING TEST")
        print("=" * 70)
        print()
        
        for user_input in test_inputs:
            print(f"Input: {user_input}")
            print("-" * 70)
            
            # Single best match
            match = matcher.match_task(user_input, tasks, threshold=0.5)
            if match:
                print(f"Best Match: [{match.task.id}] {match.task.title}")
                print(f"Confidence: {match.confidence:.0%}")
                print(f"Type: {match.match_type}")
                print(f"Reason: {match.reason}")
            else:
                print("No match found above threshold")
            
            print()
            
            # Multiple matches
            matches = matcher.match_multiple(user_input, tasks, top_n=2)
            if len(matches) > 1:
                print("Other possible matches:")
                for m in matches[1:]:
                    print(f"  - [{m.task.id}] {m.task.title} ({m.confidence:.0%})")
                print()
            
            print()
    
    elif command == 'match':
        if len(sys.argv) < 3:
            print("Error: Please provide text to match")
            return
        
        user_input = " ".join(sys.argv[2:])
        
        # Would fetch real tasks here
        print("Error: No tasks available. Use 'test' command for demo.")


if __name__ == "__main__":
    main()
