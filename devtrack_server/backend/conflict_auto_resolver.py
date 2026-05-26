"""
Automatic Conflict Resolution Module for DevTrack

This module handles automatic resolution of git merge/rebase conflicts
using intelligent strategies from git-sage.

Usage in workflows:
1. On merge/rebase failure, call detect_and_resolve()
2. It attempts smart resolution
3. Reports which conflicts were resolved and which need manual intervention
4. Stages successfully resolved files
"""

import os
import logging
import subprocess
from typing import Dict, List, Optional, Tuple

logger = logging.getLogger(__name__)

try:
    from backend.git_sage import GitOperations, ConflictResolver, ConflictAnalyzer
    HAS_GIT_SAGE = True
except ImportError:
    HAS_GIT_SAGE = False


class ConflictAutoResolver:
    """Automatically resolve conflicts when safe"""

    def __init__(self, repo_path: str = "."):
        self.repo_path = repo_path
        self.git_ops = GitOperations(cwd=repo_path) if HAS_GIT_SAGE else None
        self.resolver = ConflictResolver(strategy="smart") if HAS_GIT_SAGE else None
        self.analyzer = ConflictAnalyzer() if HAS_GIT_SAGE else None

    def detect_and_resolve(self) -> Dict[str, any]:
        """
        Detect conflicts and attempt automatic resolution.

        Returns:
            {
                "status": "success" | "partial" | "failed",
                "total_conflicts": int,
                "resolved": List[str],              # files resolved
                "unresolvable": List[str],          # files needing manual work
                "error": Optional[str],
                "summary": str                      # human-readable summary
            }
        """
        if not HAS_GIT_SAGE:
            return {
                "status": "failed",
                "error": "git-sage not available",
                "total_conflicts": 0,
                "resolved": [],
                "unresolvable": [],
                "summary": "git-sage module not installed"
            }

        try:
            # Detect conflicted files
            conflicted = self.git_ops.detect_conflicts()
            if not conflicted:
                return {
                    "status": "success",
                    "total_conflicts": 0,
                    "resolved": [],
                    "unresolvable": [],
                    "summary": "No conflicts detected"
                }

            logger.info(f"Found {len(conflicted)} conflicted files")

            resolved = []
            unresolvable = []

            # Process each conflicted file
            for conflict_file in conflicted:
                result = self._resolve_single_file(conflict_file)
                if result["success"]:
                    resolved.append(conflict_file)
                    logger.info(f"✓ Resolved: {conflict_file}")
                else:
                    unresolvable.append(conflict_file)
                    logger.warning(f"✗ Unresolvable: {conflict_file} - {result['reason']}")

            # Determine overall status
            if not unresolvable:
                status = "success"
                summary = f"✓ All {len(resolved)} conflicts resolved automatically"
            elif not resolved:
                status = "failed"
                summary = f"✗ All {len(unresolvable)} conflicts need manual resolution"
            else:
                status = "partial"
                summary = f"⚠ Resolved {len(resolved)}/{len(conflicted)} conflicts. {len(unresolvable)} need manual work"

            return {
                "status": status,
                "total_conflicts": len(conflicted),
                "resolved": resolved,
                "unresolvable": unresolvable,
                "error": None,
                "summary": summary
            }

        except Exception as e:
            logger.error(f"Error during conflict resolution: {e}")
            return {
                "status": "failed",
                "error": str(e),
                "total_conflicts": len(conflicted) if 'conflicted' in locals() else 0,
                "resolved": [],
                "unresolvable": [],
                "summary": f"Error: {str(e)}"
            }

    def _resolve_single_file(self, file_path: str) -> Dict[str, any]:
        """
        Attempt to resolve a single conflicted file.

        Returns:
            {"success": bool, "reason": str}
        """
        try:
            # Read the file with conflicts
            full_path = os.path.join(self.repo_path, file_path)
            with open(full_path, 'r', encoding='utf-8', errors='replace') as f:
                content = f.read()

            if "<<<<<<< " not in content:
                return {"success": False, "reason": "No conflict markers found"}

            # Analyze conflicts
            conflict_count = self.analyzer.count_conflicts(content)
            logger.debug(f"File {file_path}: {conflict_count} conflicts")

            # Try to resolve
            resolved, has_unresolvable = self.resolver.resolve_file(content)

            if has_unresolvable:
                # Some conflicts couldn't be auto-resolved
                unresolvable_count = self.resolver.extract_unresolvable_conflicts(resolved)
                return {
                    "success": False,
                    "reason": f"{len(unresolvable_count)} unresolvable conflicts"
                }

            # All conflicts resolved - write back
            with open(full_path, 'w', encoding='utf-8') as f:
                f.write(resolved)

            # Stage the resolved file
            from backend.config import http_timeout_short
            subprocess.run(
                ["git", "add", file_path],
                cwd=self.repo_path,
                capture_output=True,
                timeout=http_timeout_short()
            )

            return {"success": True, "reason": "Resolved and staged"}

        except Exception as e:
            return {"success": False, "reason": str(e)}

    def get_conflict_summary(self, file_path: str) -> Optional[str]:
        """Get human-readable summary of conflicts in a file"""
        try:
            full_path = os.path.join(self.repo_path, file_path)
            with open(full_path, 'r', encoding='utf-8', errors='replace') as f:
                content = f.read()

            return self.analyzer.conflict_summary(content)
        except Exception as e:
            logger.error(f"Error analyzing conflict summary: {e}")
            return None

    def get_conflict_sections(self, file_path: str) -> List[Dict]:
        """Get detailed conflict sections with context"""
        try:
            full_path = os.path.join(self.repo_path, file_path)
            with open(full_path, 'r', encoding='utf-8', errors='replace') as f:
                content = f.read()

            return self.analyzer.get_conflicted_sections(content)
        except Exception as e:
            logger.error(f"Error analyzing conflict sections: {e}")
            return []


def auto_resolve_merge_conflicts(repo_path: str = ".") -> Dict[str, any]:
    """
    Convenience function to detect and resolve conflicts in a repo.

    Returns:
        Same as ConflictAutoResolver.detect_and_resolve()
    """
    resolver = ConflictAutoResolver(repo_path=repo_path)
    return resolver.detect_and_resolve()


def get_conflict_report(repo_path: str = ".") -> str:
    """
    Get a detailed report of conflicts for manual resolution.

    Returns:
        Formatted string describing all conflicts
    """
    resolver = ConflictAutoResolver(repo_path=repo_path)
    conflicted = resolver.git_ops.detect_conflicts() if resolver.git_ops else []

    if not conflicted:
        return "No conflicts found"

    lines = [f"Found {len(conflicted)} conflicted files:\n"]

    for file_path in conflicted:
        summary = resolver.get_conflict_summary(file_path)
        lines.append(f"\n📄 {file_path}")
        if summary:
            lines.append(f"   {summary}")

        sections = resolver.get_conflict_sections(file_path)
        if sections:
            for section in sections[:3]:  # Show first 3 conflicts
                lines.append(
                    f"   Conflict #{section['number']}: "
                    f"{section['current_lines']} vs {section['incoming_lines']} lines "
                    f"({section['branch_from']} → {section['branch_to']})"
                )

    return "\n".join(lines)
