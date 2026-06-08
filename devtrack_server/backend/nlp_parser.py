"""
NLP Task Parser — LLM-first implementation.

Extracts structured task information from developer work-update text using
the configured LLM provider for rich semantic understanding. Falls back to
pure regex extraction when the LLM is unavailable, so the server starts
and operates without any AI dependencies installed.

spaCy has been removed. The LLM call covers everything spaCy previously
provided (NER, POS-based verb lemmatization, pattern matching) with better
accuracy and zero model-file overhead.
"""

import json
import re
import logging
from typing import Dict, List, Optional, Tuple
from dataclasses import dataclass

logger = logging.getLogger(__name__)

# Optional work-update enricher (git branch/PR context injection)
try:
    from backend.work_update_enhancer import get_work_context
    HAS_WORK_ENHANCER = True
except ImportError:
    HAS_WORK_ENHANCER = False


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
    git_context: Optional[Dict] = None

    def __post_init__(self):
        if self.entities is None:
            self.entities = {}
        if self.git_context is None:
            self.git_context = {}

    def to_dict(self) -> Dict:
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
            "git_context": self.git_context,
        }


# LLM prompt that requests a strict JSON response.
# The double-braces escape the f-string so the schema braces are literal.
_LLM_PROMPT_TEMPLATE = """\
You are a developer work-update parser. Extract structured information from the text.
Return ONLY a JSON object — no markdown fences, no explanation, nothing else.

JSON schema (use null for any field you cannot determine):
{{
  "ticket_id": "<ticket id e.g. AB-123 PROJ-456 #42, or null>",
  "project": "<project or product name, or null>",
  "action_verb": "<one verb: fixed completed working implementing started began blocked waiting reviewing testing deployed merged, or null>",
  "status": "<one of: completed in_progress started blocked waiting in_review testing, or null>",
  "time_spent": "<e.g. 2h 30m 1.5d, or null>",
  "time_estimate": "<e.g. 3h, or null>",
  "description": "<clean work description without ticket id or time info>"
}}

Examples:
Input: "Fixed login bug for Project Alpha AB-123, spent 2 hours"
Output: {{"ticket_id":"AB-123","project":"Project Alpha","action_verb":"fixed","status":"completed","time_spent":"2h","time_estimate":null,"description":"Fixed login bug"}}

Input: "Working on PROJ-456 implementing new API endpoint"
Output: {{"ticket_id":"PROJ-456","project":null,"action_verb":"working","status":"in_progress","time_spent":null,"time_estimate":null,"description":"Implementing new API endpoint"}}

Input: {text}
Output:"""


class NLPTaskParser:
    """LLM-first task parser with regex fallback.

    When use_ollama=True (default), sends the text to the configured LLM
    provider and parses the JSON response. Any field the LLM leaves null
    is filled in by the regex pipeline. If the LLM is unavailable or
    returns unparseable output, all fields come from regex — no crash.
    """

    # Ticket number patterns (various formats)
    TICKET_PATTERNS = [
        r'#(\d+)',                          # #123
        r'([A-Z]{2,10}-\d+)',              # PROJ-456, PA-123
        r'([A-Z]+\d+)',                    # ABC123
        r'ticket[:\s]+(\d+)',              # ticket: 123
        r'issue[:\s]+(\d+)',               # issue: 123
    ]

    # Time patterns
    TIME_PATTERNS = [
        r'(\d+\.?\d*)\s*h(?:our)?s?',     # 2h, 2.5 hours
        r'(\d+)\s*m(?:in)?(?:ute)?s?',    # 30min, 30 minutes
        r'(\d+\.?\d*)\s*d(?:ay)?s?',      # 2d, 1.5 days
    ]

    # Action verbs → status mapping
    ACTION_VERBS = {
        'completed': 'completed', 'finished': 'completed', 'done': 'completed',
        'fixed': 'completed', 'resolved': 'completed', 'merged': 'completed',
        'deployed': 'completed', 'released': 'completed', 'closed': 'completed',
        'working': 'in_progress', 'implementing': 'in_progress', 'developing': 'in_progress',
        'coding': 'in_progress', 'building': 'in_progress', 'creating': 'in_progress',
        'writing': 'in_progress', 'updating': 'in_progress', 'refactoring': 'in_progress',
        'debugging': 'in_progress',
        'started': 'started', 'began': 'started', 'initiated': 'started',
        'kicked off': 'started',
        'blocked': 'blocked', 'waiting': 'waiting', 'stuck': 'blocked',
        'reviewing': 'in_review', 'testing': 'testing', 'qa': 'testing',
    }

    PROJECT_INDICATORS = ['project', 'for', 'on', 'in']

    def __init__(self, use_ollama: bool = True):
        self.use_ollama = use_ollama
        self.ticket_regex = [re.compile(p, re.IGNORECASE) for p in self.TICKET_PATTERNS]
        self.time_regex = [re.compile(p, re.IGNORECASE) for p in self.TIME_PATTERNS]

    # -- LLM extraction -------------------------------------------------------

    def _try_llm_parse(self, text: str) -> Optional[Dict]:
        """Call the configured LLM provider for structured JSON extraction.

        Returns a dict of extracted fields, or None if the LLM is unavailable,
        returns a non-JSON response, or raises any exception. Callers treat
        None as a signal to fall back to pure regex.
        """
        try:
            from backend.llm import get_provider
            from backend.llm.base import LLMOptions
        except ImportError:
            return None

        try:
            provider = get_provider()
            prompt = _LLM_PROMPT_TEMPLATE.format(text=text)
            result = provider.generate(
                prompt,
                LLMOptions(
                    temperature=0.1,
                    max_tokens=300,
                    extra={"format": "json"},  # Ollama JSON mode; ignored by other providers
                ),
            )
            if not result:
                return None
            # Strip markdown code fences some models add
            cleaned = re.sub(r"```(?:json)?\s*|\s*```", "", result).strip()
            # Extract the first JSON object if the model added surrounding text
            m = re.search(r'\{.*\}', cleaned, re.DOTALL)
            cleaned = m.group(0) if m else cleaned
            data = json.loads(cleaned)
            # Normalise "null"/"none"/"" strings that some models emit
            return {
                k: (None if str(v).lower() in ("null", "none", "") else v)
                for k, v in data.items()
            }
        except Exception as e:
            logger.debug(f"LLM NLP extraction failed (will use regex fallback): {e}")
            return None

    # -- public API -----------------------------------------------------------

    def parse(self, text: str, repo_path: str = ".") -> ParsedTask:
        """Parse a developer work-update into a structured ParsedTask."""
        logger.info(f"Parsing text: {text}")
        task = ParsedTask(raw_text=text)

        # Git context injection (branch, PR metadata)
        if HAS_WORK_ENHANCER:
            try:
                task.git_context = get_work_context(repo_path) or {}
                if task.git_context.get('branch'):
                    logger.debug(f"Git context: {task.git_context.get('branch')}")
            except Exception as e:
                logger.debug(f"Error getting git context: {e}")
                task.git_context = {}

        # LLM-first extraction
        llm = self._try_llm_parse(text) if self.use_ollama else None

        if llm:
            # LLM result takes precedence; regex fills any nulls
            task.ticket_id = llm.get("ticket_id") or self._extract_ticket_number(text)
            task.project = llm.get("project")
            task.action_verb = llm.get("action_verb")
            task.status = llm.get("status") or "in_progress"
            task.time_spent = llm.get("time_spent")
            task.time_estimate = llm.get("time_estimate")
            task.description = llm.get("description") or text
            task.entities = {}
        else:
            # Pure regex fallback — no LLM required
            task.ticket_id = self._extract_ticket_number(text)
            time_info = self._extract_time(text)
            task.time_estimate = time_info.get('estimate')
            task.time_spent = time_info.get('spent')
            action, status = self._extract_action_verb(text)
            task.action_verb = action
            task.status = status or 'in_progress'
            task.entities = {}
            task.project = self._extract_project_regex(text)
            task.description = self._build_description(text, task)

        # Augment ticket_id from git PR number if not found in text
        if not task.ticket_id and task.git_context.get('branch'):
            pr_number = task.git_context['branch'].get('issue_number')
            if pr_number:
                task.ticket_id = pr_number
                logger.debug(f"Extracted ticket from git context: {task.ticket_id}")

        # Append branch name to description for traceability
        if task.git_context.get('branch') and task.description:
            branch_info = task.git_context['branch'].get('branch', '')
            if branch_info and branch_info not in task.description:
                task.description = f"{task.description} (on {branch_info})"

        task.confidence = self._calculate_confidence(task)
        logger.info(f"Parsed result: {task.to_dict()}")
        return task

    def parse_batch(self, texts: List[str]) -> List[ParsedTask]:
        return [self.parse(text) for text in texts]

    # -- regex helpers --------------------------------------------------------

    def _extract_ticket_number(self, text: str) -> Optional[str]:
        for regex in self.ticket_regex:
            match = regex.search(text)
            if match:
                ticket = match.group(1) if len(match.groups()) > 0 else match.group(0)
                logger.debug(f"Found ticket: {ticket}")
                return ticket
        return None

    def _extract_time(self, text: str) -> Dict[str, Optional[str]]:
        result: Dict[str, Optional[str]] = {'estimate': None, 'spent': None}
        spent_match = re.search(
            r'(?:spent|took)\s+(\d+\.?\d*\s*(?:h|hour|min|day)s?)', text, re.IGNORECASE
        )
        if spent_match:
            result['spent'] = self._normalize_time(spent_match.group(1))
        for regex in self.time_regex:
            matches = regex.findall(text)
            if matches:
                time_str = self._normalize_time(f"{matches[0]} {regex.pattern.split('?')[0][-1]}")
                if result['spent'] is None:
                    result['spent'] = time_str
                else:
                    result['estimate'] = time_str
                break
        return result

    def _normalize_time(self, time_str: str) -> str:
        match = re.search(r'(\d+\.?\d*)\s*([hdm])', time_str, re.IGNORECASE)
        if match:
            value, unit = match.groups()
            return f"{value}{unit.lower()}"
        return time_str

    def _extract_action_verb(self, text: str) -> Tuple[Optional[str], Optional[str]]:
        text_lower = text.lower()
        for verb, status in self.ACTION_VERBS.items():
            if verb in text_lower:
                logger.debug(f"Found action: {verb} -> status: {status}")
                return verb, status
        return None, None

    def _extract_project_regex(self, text: str) -> Optional[str]:
        for indicator in self.PROJECT_INDICATORS:
            pattern = rf'{indicator}\s+([A-Z][A-Za-z0-9_\-]+)'
            match = re.search(pattern, text)
            if match:
                project = match.group(1)
                logger.debug(f"Found project: {project}")
                return project
        return None

    def _build_description(self, text: str, task: ParsedTask) -> str:
        description = text
        if task.ticket_id:
            for regex in self.ticket_regex:
                description = regex.sub('', description)
        for regex in self.time_regex:
            description = regex.sub('', description)
        if task.project:
            for indicator in self.PROJECT_INDICATORS:
                description = re.sub(
                    rf'{indicator}\s+{re.escape(task.project)}', '',
                    description, flags=re.IGNORECASE
                )
        description = re.sub(r'\s+', ' ', description).strip()
        if len(description) < 10:
            description = text
        return description

    def _calculate_confidence(self, task: ParsedTask) -> float:
        confidence = 0.0
        if task.ticket_id:
            confidence += 0.3
        if task.project:
            confidence += 0.2
        if task.action_verb:
            confidence += 0.2
        if task.time_spent or task.time_estimate:
            confidence += 0.15
        if task.entities:
            confidence += 0.1 * min(len(task.entities), 1.5)
        if len(task.description) > 10:
            confidence += 0.05
        return min(confidence, 1.0)


def parse_task(text: str, use_ollama: bool = True) -> ParsedTask:
    """Quick helper to parse a single task."""
    return NLPTaskParser(use_ollama=use_ollama).parse(text)


if __name__ == "__main__":
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
    for i, text in enumerate(examples, 1):
        print(f"\nExample {i}: {text}")
        print("-" * 60)
        task = parser.parse(text)
        print(f"Project:     {task.project}")
        print(f"Ticket:      {task.ticket_id}")
        print(f"Action:      {task.action_verb}")
        print(f"Status:      {task.status}")
        print(f"Time Spent:  {task.time_spent}")
        print(f"Description: {task.description}")
        print(f"Confidence:  {task.confidence:.2f}")
