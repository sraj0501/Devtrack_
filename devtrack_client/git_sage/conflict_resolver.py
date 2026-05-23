"""Intelligent conflict resolution utilities."""
import re
from typing import Dict, List, Optional, Tuple


class ConflictMarker:
    """Parse and represent a conflict marker."""

    def __init__(self, content: str):
        """Parse conflict marker: <<<<<<< HEAD ... ======= ... >>>>>>>"""
        self.raw = content
        self.current = ""
        self.incoming = ""
        self.branch = ""
        self._parse()

    def _parse(self):
        """Extract current and incoming content."""
        # Pattern: <<<<<<< HEAD\n(.*)\n=======\n(.*)\n>>>>>>> branch
        pattern = r"<<<<<<< (.+?)\n(.*?)\n=======\n(.*?)\n>>>>>>> (.+?)(?:\n|$)"
        match = re.search(pattern, self.raw, re.DOTALL)
        if match:
            self.branch_head = match.group(1)
            self.current = match.group(2)
            self.incoming = match.group(3)
            self.branch = match.group(4)

    def lines_current(self) -> List[str]:
        """Get current side lines."""
        return self.current.strip().split("\n")

    def lines_incoming(self) -> List[str]:
        """Get incoming side lines."""
        return self.incoming.strip().split("\n")

    def is_addition_only(self) -> bool:
        """Check if one side is addition (other is empty/whitespace)."""
        current_clean = self.current.strip()
        incoming_clean = self.incoming.strip()
        return not current_clean or not incoming_clean

    def is_same(self) -> bool:
        """Check if both sides are identical."""
        return self.current.strip() == self.incoming.strip()

    def is_adjacent_changes(self) -> bool:
        """Check if changes don't overlap (can be merged safely)."""
        current_lines = self.lines_current()
        incoming_lines = self.lines_incoming()

        # If one side is empty, it's safe
        if not current_lines or not incoming_lines:
            return True

        # Check if there's overlap in common tokens
        current_set = set(" ".join(current_lines).split())
        incoming_set = set(" ".join(incoming_lines).split())

        # If sets don't overlap much, consider them adjacent
        overlap = current_set & incoming_set
        return len(overlap) < 3  # Very small overlap


class ConflictResolver:
    """Intelligent conflict resolver."""

    def __init__(self, strategy: str = "smart"):
        """Initialize resolver with strategy: smart, ours, theirs, both."""
        self.strategy = strategy

    def resolve_file(self, content: str) -> Tuple[str, bool]:
        """
        Resolve all conflicts in file content.
        Returns (resolved_content, has_unresolvable)
        """
        if "<<<<<<< " not in content:
            return content, False

        resolved = content
        has_unresolvable = False
        matches = list(re.finditer(r"<<<<<<< .+?\n(.*?)\n=======\n(.*?)\n>>>>>>> .+?(?:\n|$)",
                                    content, re.DOTALL))

        for match in reversed(matches):  # Process from end to avoid offset issues
            conflict_text = match.group(0)
            marker = ConflictMarker(conflict_text)

            resolution = self._resolve_marker(marker)
            if resolution is None:
                has_unresolvable = True
                resolution = conflict_text  # Keep original

            resolved = resolved[:match.start()] + resolution + resolved[match.end():]

        return resolved, has_unresolvable

    def _resolve_marker(self, marker: ConflictMarker) -> Optional[str]:
        """Resolve a single conflict marker using configured strategy."""
        if self.strategy == "ours":
            return marker.current

        if self.strategy == "theirs":
            return marker.incoming

        if self.strategy == "both":
            return f"{marker.current}\n{marker.incoming}"

        # Smart resolution
        if marker.is_same():
            return marker.current  # Identical, use either

        if marker.is_addition_only():
            # One side added, keep the non-empty one
            current = marker.current.strip()
            incoming = marker.incoming.strip()
            return incoming if current == "" else current

        if marker.is_adjacent_changes():
            # Non-overlapping changes, merge both
            return f"{marker.current}\n{marker.incoming}"

        # Can't auto-resolve
        return None

    def extract_unresolvable_conflicts(self, content: str) -> List[Dict[str, str]]:
        """Extract conflicts that couldn't be resolved."""
        conflicts = []
        pattern = r"<<<<<<< (.+?)\n(.*?)\n=======\n(.*?)\n>>>>>>> (.+?)(?:\n|$)"

        for match in re.finditer(pattern, content, re.DOTALL):
            marker = ConflictMarker(match.group(0))
            if not self._can_resolve(marker):
                conflicts.append({
                    "marker": marker.raw[:100],  # First 100 chars
                    "current": marker.current[:200],
                    "incoming": marker.incoming[:200],
                    "branch": marker.branch
                })
        return conflicts

    def _can_resolve(self, marker: ConflictMarker) -> bool:
        """Check if a marker can be auto-resolved."""
        if marker.is_same():
            return True
        if marker.is_addition_only():
            return True
        if self.strategy in ("ours", "theirs", "both"):
            return True
        if marker.is_adjacent_changes():
            return True
        return False

    def suggest_resolution_strategy(self, content: str) -> str:
        """Suggest best resolution strategy for file."""
        pattern = r"<<<<<<< .+?\n(.*?)\n=======\n(.*?)\n>>>>>>> .+?(?:\n|$)"
        matches = list(re.finditer(pattern, content, re.DOTALL))

        if not matches:
            return "no_conflicts"

        resolvable = 0
        for match in matches:
            marker = ConflictMarker(match.group(0))
            if self._can_resolve(marker):
                resolvable += 1

        if resolvable == len(matches):
            return "auto"  # Can auto-resolve all
        elif resolvable > 0:
            return "mixed"  # Some can be auto-resolved
        else:
            return "manual"  # All need manual resolution


class ConflictAnalyzer:
    """Analyze conflicts to help understand them better."""

    @staticmethod
    def count_conflicts(content: str) -> int:
        """Count number of conflict markers."""
        return len(re.findall(r"<<<<<<< ", content))

    @staticmethod
    def conflict_summary(content: str) -> str:
        """Get human-readable summary of conflicts."""
        count = ConflictAnalyzer.count_conflicts(content)
        if count == 0:
            return "No conflicts"
        elif count == 1:
            return "1 conflict"
        else:
            return f"{count} conflicts"

    @staticmethod
    def get_conflicted_sections(content: str) -> List[Dict[str, any]]:
        """Extract conflict sections with context."""
        sections = []
        pattern = r"<<<<<<< (.+?)\n(.*?)\n=======\n(.*?)\n>>>>>>> (.+?)(?:\n|$)"

        for i, match in enumerate(re.finditer(pattern, content, re.DOTALL)):
            # Get context (5 lines before and after)
            start = max(0, match.start() - 200)
            end = min(len(content), match.end() + 200)

            context_before = content[start:match.start()].split("\n")[-3:]
            context_after = content[match.end():end].split("\n")[:3]

            sections.append({
                "number": i + 1,
                "branch_from": match.group(1),
                "branch_to": match.group(4),
                "current_lines": len(match.group(2).split("\n")),
                "incoming_lines": len(match.group(3).split("\n")),
                "context_before": context_before,
                "context_after": context_after,
            })

        return sections

    @staticmethod
    def suggest_merge_direction(
        content_our: str,
        content_theirs: str,
        conflict_count: int
    ) -> str:
        """Suggest whether to keep ours or theirs based on content analysis."""
        # Simple heuristic: prefer the shorter version for conflict resolution
        our_len = len(content_our.strip())
        theirs_len = len(content_theirs.strip())

        if our_len == 0:
            return "theirs"
        if theirs_len == 0:
            return "ours"
        if abs(our_len - theirs_len) > our_len * 0.5:  # >50% difference
            return "theirs" if theirs_len < our_len else "ours"
        return "both"  # Merge both if similar length
