"""
NLP Task Parser using spaCy

This module parses natural language text to extract task information including:
- Project names
- Ticket numbers (#123, PROJ-456, etc.)
- Time estimates (2h, 30min, 1.5 hours)
- Action verbs (completed, started, working on, fixed, etc.)
- Task descriptions
- Status indicators

Uses spaCy for NER and pattern matching, with Ollama for enhancement.
"""

import re
import logging
from typing import Dict, List, Optional, Tuple
from dataclasses import dataclass
import spacy
from spacy.matcher import Matcher
from spacy.tokens import Doc

# Set up logging
logger = logging.getLogger(__name__)

# Try to import work update enhancer
try:
    from backend.work_update_enhancer import get_work_context
    HAS_WORK_ENHANCER = True
except ImportError:
    HAS_WORK_ENHANCER = False

# Try to load spaCy model
try:
    nlp = spacy.load("en_core_web_sm")
    logger.info("Loaded spaCy model: en_core_web_sm")
except OSError:
    logger.warning("spaCy model not found. Please install: python -m spacy download en_core_web_sm")
    nlp = None


@dataclass
class ParsedTask:
    """Represents a parsed task from natural language text"""
    raw_text: str
    project: Optional[str] = None
    ticket_id: Optional[str] = None
    description: str = ""
    action_verb: Optional[str] = None
    time_estimate: Optional[str] = None
    time_spent: Optional[str] = None
    status: Optional[str] = None
    entities: Dict[str, List[str]] = None
    confidence: float = 0.0
    git_context: Optional[Dict] = None  # NEW: Git branch/PR context

    def __post_init__(self):
        if self.entities is None:
            self.entities = {}
        if self.git_context is None:
            self.git_context = {}
    
    def to_dict(self) -> Dict:
        """Convert to dictionary"""
        return {
            "raw_text": self.raw_text,
            "project": self.project,
            "ticket_id": self.ticket_id,
            "description": self.description,
            "action_verb": self.action_verb,
            "time_estimate": self.time_estimate,
            "time_spent": self.time_spent,
            "status": self.status,
            "entities": self.entities,
            "confidence": self.confidence,
            "git_context": self.git_context
        }


class NLPTaskParser:
    """NLP-based task parser using spaCy"""
    
    # Ticket number patterns (various formats)
    TICKET_PATTERNS = [
        r'#(\d+)',                          # #123
        r'([A-Z]{2,10}-\d+)',              # PROJ-456, PA-123
        r'([A-Z]+\d+)',                    # ABC123
        r'ticket[:\s]+(\d+)',              # ticket: 123, ticket 123
        r'issue[:\s]+(\d+)',               # issue: 123
    ]
    
    # Time patterns
    TIME_PATTERNS = [
        r'(\d+\.?\d*)\s*h(?:our)?s?',     # 2h, 2.5 hours
        r'(\d+)\s*m(?:in)?(?:ute)?s?',    # 30min, 30 minutes
        r'(\d+\.?\d*)\s*d(?:ay)?s?',      # 2d, 1.5 days
    ]
    
    # Action verbs and their status mappings
    ACTION_VERBS = {
        # Completed actions
        'completed': 'completed',
        'finished': 'completed',
        'done': 'completed',
        'fixed': 'completed',
        'resolved': 'completed',
        'merged': 'completed',
        'deployed': 'completed',
        'released': 'completed',
        'closed': 'completed',
        
        # In progress actions
        'working': 'in_progress',
        'implementing': 'in_progress',
        'developing': 'in_progress',
        'coding': 'in_progress',
        'building': 'in_progress',
        'creating': 'in_progress',
        'writing': 'in_progress',
        'updating': 'in_progress',
        'refactoring': 'in_progress',
        'debugging': 'in_progress',
        
        # Starting actions
        'started': 'started',
        'began': 'started',
        'initiated': 'started',
        'kicked off': 'started',
        
        # Blocked/waiting
        'blocked': 'blocked',
        'waiting': 'waiting',
        'stuck': 'blocked',
        
        # Review/testing
        'reviewing': 'in_review',
        'testing': 'testing',
        'qa': 'testing',
    }
    
    # Project name indicators
    PROJECT_INDICATORS = ['project', 'for', 'on', 'in']
    
    def __init__(self, use_ollama: bool = True):
        """
        Initialize NLP task parser
        
        Args:
            use_ollama: Whether to use Ollama for enhancement (only option for AI)
        """
        self.use_ollama = use_ollama
        
        if nlp is None:
            raise RuntimeError("spaCy model not loaded. Install with: python -m spacy download en_core_web_sm")
        
        # Compile regex patterns
        self.ticket_regex = [re.compile(p, re.IGNORECASE) for p in self.TICKET_PATTERNS]
        self.time_regex = [re.compile(p, re.IGNORECASE) for p in self.TIME_PATTERNS]
        
        # Create spaCy matcher for patterns
        self.matcher = Matcher(nlp.vocab)
        self._add_patterns()
    
    def _add_patterns(self):
        """Add custom patterns to spaCy matcher"""
        # Pattern for "working on X"
        working_on_pattern = [
            {"LEMMA": {"IN": ["work", "working"]}},
            {"LOWER": "on"},
            {"POS": {"IN": ["NOUN", "PROPN"]}, "OP": "+"}
        ]
        self.matcher.add("WORKING_ON", [working_on_pattern])
        
        # Pattern for "fixed X"
        fixed_pattern = [
            {"LEMMA": {"IN": ["fix", "resolve", "complete"]}},
            {"POS": {"IN": ["NOUN", "PROPN"]}, "OP": "+"}
        ]
        self.matcher.add("FIXED", [fixed_pattern])
    
    def parse(self, text: str, repo_path: str = ".") -> ParsedTask:
        """
        Parse natural language text to extract task information

        Args:
            text: The text to parse
            repo_path: Path to git repo for context extraction (optional)

        Returns:
            ParsedTask object with extracted information
        """
        logger.info(f"Parsing text: {text}")

        # Create parsed task
        task = ParsedTask(raw_text=text)

        # Extract git context (branch, PR metadata, changes)
        if HAS_WORK_ENHANCER:
            try:
                task.git_context = get_work_context(repo_path) or {}
                if task.git_context.get('branch'):
                    logger.debug(f"Git context: {task.git_context.get('branch')}")
            except Exception as e:
                logger.debug(f"Error getting git context: {e}")
                task.git_context = {}

        # Process with spaCy
        doc = nlp(text)

        # Extract ticket numbers
        task.ticket_id = self._extract_ticket_number(text)

        # Try to extract ticket from git context (PR number)
        if not task.ticket_id and task.git_context.get('branch'):
            pr_number = task.git_context['branch'].get('issue_number')
            if pr_number:
                task.ticket_id = pr_number
                logger.debug(f"Extracted ticket from git context: {task.ticket_id}")

        # Extract time information
        time_info = self._extract_time(text)
        task.time_estimate = time_info.get('estimate')
        task.time_spent = time_info.get('spent')

        # Extract action verb and status
        action, status = self._extract_action_and_status(doc, text)
        task.action_verb = action
        task.status = status or 'in_progress'  # Default status

        # Extract entities
        task.entities = self._extract_entities(doc)

        # Extract project name
        task.project = self._extract_project(text, doc, task.entities)

        # Extract description
        task.description = self._extract_description(text, doc, task)

        # Enhance description with git context if available
        if task.git_context.get('branch') and task.description:
            branch_info = task.git_context['branch'].get('branch', '')
            if branch_info and branch_info not in task.description:
                task.description = f"{task.description} (on {branch_info})"

        # Calculate confidence
        task.confidence = self._calculate_confidence(task)

        logger.info(f"Parsed result: {task.to_dict()}")

        return task
    
    def _extract_ticket_number(self, text: str) -> Optional[str]:
        """Extract ticket number from text"""
        for regex in self.ticket_regex:
            match = regex.search(text)
            if match:
                ticket = match.group(1) if len(match.groups()) > 0 else match.group(0)
                logger.debug(f"Found ticket: {ticket}")
                return ticket
        return None
    
    def _extract_time(self, text: str) -> Dict[str, Optional[str]]:
        """Extract time estimates and time spent"""
        result = {'estimate': None, 'spent': None}
        
        # Look for "spent X" or "took X"
        spent_match = re.search(r'(?:spent|took)\s+(\d+\.?\d*\s*(?:h|hour|min|day)s?)', text, re.IGNORECASE)
        if spent_match:
            result['spent'] = self._normalize_time(spent_match.group(1))
        
        # Look for general time mentions
        for regex in self.time_regex:
            matches = regex.findall(text)
            if matches:
                # If we already have spent, this is estimate
                time_str = self._normalize_time(f"{matches[0]} {regex.pattern.split('?')[0][-1]}")
                if result['spent'] is None:
                    result['spent'] = time_str
                else:
                    result['estimate'] = time_str
                break
        
        return result
    
    def _normalize_time(self, time_str: str) -> str:
        """Normalize time string to standard format"""
        # Extract number and unit
        match = re.search(r'(\d+\.?\d*)\s*([hdm])', time_str, re.IGNORECASE)
        if match:
            value, unit = match.groups()
            unit_map = {'h': 'h', 'd': 'd', 'm': 'm'}
            return f"{value}{unit_map.get(unit.lower(), unit)}"
        return time_str
    
    def _extract_action_and_status(self, doc: Doc, text: str) -> Tuple[Optional[str], Optional[str]]:
        """Extract action verb and infer status"""
        text_lower = text.lower()
        
        # Check for action verbs
        for verb, status in self.ACTION_VERBS.items():
            if verb in text_lower:
                logger.debug(f"Found action: {verb} -> status: {status}")
                return verb, status
        
        # Check spaCy verbs
        for token in doc:
            if token.pos_ == "VERB":
                lemma = token.lemma_.lower()
                if lemma in self.ACTION_VERBS:
                    status = self.ACTION_VERBS[lemma]
                    logger.debug(f"Found verb: {lemma} -> status: {status}")
                    return lemma, status
        
        return None, None
    
    def _extract_entities(self, doc: Doc) -> Dict[str, List[str]]:
        """Extract named entities from text"""
        entities = {}
        
        for ent in doc.ents:
            if ent.label_ not in entities:
                entities[ent.label_] = []
            entities[ent.label_].append(ent.text)
        
        logger.debug(f"Extracted entities: {entities}")
        return entities
    
    def _extract_project(self, text: str, doc: Doc, entities: Dict) -> Optional[str]:
        """Extract project name from text"""
        text_lower = text.lower()
        
        # Look for explicit project mentions
        for indicator in self.PROJECT_INDICATORS:
            pattern = rf'{indicator}\s+([A-Z][A-Za-z0-9_\-]+)'
            match = re.search(pattern, text)
            if match:
                project = match.group(1)
                logger.debug(f"Found project: {project}")
                return project
        
        # Look in entities
        for label in ['ORG', 'PRODUCT']:
            if label in entities and entities[label]:
                project = entities[label][0]
                logger.debug(f"Found project from entity: {project}")
                return project
        
        # Look for capitalized words that might be project names
        for token in doc:
            if token.is_title and len(token.text) > 3 and token.pos_ in ['PROPN', 'NOUN']:
                logger.debug(f"Found potential project: {token.text}")
                return token.text
        
        return None
    
    def _extract_description(self, text: str, doc: Doc, task: ParsedTask) -> str:
        """Extract task description, removing ticket numbers and time info"""
        description = text
        
        # Remove ticket number
        if task.ticket_id:
            for regex in self.ticket_regex:
                description = regex.sub('', description)
        
        # Remove time information
        for regex in self.time_regex:
            description = regex.sub('', description)
        
        # Remove project indicators
        for indicator in self.PROJECT_INDICATORS:
            if task.project:
                description = re.sub(rf'{indicator}\s+{re.escape(task.project)}', '', description, flags=re.IGNORECASE)
        
        # Clean up
        description = re.sub(r'\s+', ' ', description).strip()
        
        # If too short, use full text
        if len(description) < 10:
            description = text
        
        return description
    
    def _calculate_confidence(self, task: ParsedTask) -> float:
        """Calculate confidence score for the parse"""
        confidence = 0.0
        
        # Has ticket number
        if task.ticket_id:
            confidence += 0.3
        
        # Has project
        if task.project:
            confidence += 0.2
        
        # Has action verb
        if task.action_verb:
            confidence += 0.2
        
        # Has time information
        if task.time_spent or task.time_estimate:
            confidence += 0.15
        
        # Has entities
        if task.entities:
            confidence += 0.1 * min(len(task.entities), 1.5)
        
        # Has description
        if len(task.description) > 10:
            confidence += 0.05
        
        return min(confidence, 1.0)
    
    def parse_batch(self, texts: List[str]) -> List[ParsedTask]:
        """Parse multiple texts in batch"""
        return [self.parse(text) for text in texts]


# Helper function for quick parsing
def parse_task(text: str, use_ollama: bool = True) -> ParsedTask:
    """
    Quick helper to parse a single task
    
    Args:
        text: The text to parse
        use_ollama: Whether to use Ollama for enhancement
        
    Returns:
        ParsedTask object
    """
    parser = NLPTaskParser(use_ollama=use_ollama)
    return parser.parse(text)


# Example usage
if __name__ == "__main__":
    # Example texts
    examples = [
        "Fixed login bug for Project Alpha #123, spent 2 hours",
        "Working on PROJ-456 implementing new API endpoint",
        "Completed Azure DevOps integration, ticket PA-789",
        "Started debugging authentication issue, estimated 3h",
        "Blocked on JIRA-321 waiting for backend team",
    ]
    
    parser = NLPTaskParser(use_ollama=True)
    
    print("NLP Task Parser Examples")
    print("=" * 60)
    print()
    
    for i, text in enumerate(examples, 1):
        print(f"Example {i}: {text}")
        print("-" * 60)
        
        task = parser.parse(text)
        print(f"Project:     {task.project}")
        print(f"Ticket:      {task.ticket_id}")
        print(f"Action:      {task.action_verb}")
        print(f"Status:      {task.status}")
        print(f"Time Spent:  {task.time_spent}")
        print(f"Description: {task.description}")
        print(f"Confidence:  {task.confidence:.2f}")
        print()
