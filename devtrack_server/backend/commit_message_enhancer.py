#!/usr/bin/env python3
"""
Git Commit Message Enhancer

This script is called by git's prepare-commit-msg hook to enhance commit messages
using AI analysis of staged changes.
"""

import os
import sys
import subprocess
import logging
from pathlib import Path
from typing import Optional

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Try to import git-sage utilities
try:
    from backend.git_sage import GitOperations, PRFinder
    HAS_GIT_SAGE = True
except ImportError:
    HAS_GIT_SAGE = False
    GitOperations = None
    PRFinder = None


class CommitMessageEnhancer:
    """Enhances git commit messages using AI analysis of staged changes."""

    def __init__(self, provider=None):
        self._provider = provider  # None = lazy init on first use

    def _get_provider(self):
        if self._provider is None:
            from backend.llm import get_provider
            self._provider = get_provider()
        return self._provider
        
    def get_staged_diff(self, repo_path: str) -> Optional[str]:
        """Get diff of staged changes"""
        try:
            # Get stats
            from backend.config import http_timeout_short
            timeout_secs = http_timeout_short()

            result = subprocess.run(
                ["git", "diff", "--cached", "--stat"],
                cwd=repo_path,
                capture_output=True,
                text=True,
                timeout=timeout_secs
            )

            if result.returncode != 0:
                logger.error(f"Failed to get diff stats: {result.stderr}")
                return None

            stats = result.stdout.strip()

            # Get actual diff (limited)
            result = subprocess.run(
                ["git", "diff", "--cached", "--unified=3"],
                cwd=repo_path,
                capture_output=True,
                text=True,
                timeout=timeout_secs
            )
            
            if result.returncode != 0:
                return stats
            
            diff = result.stdout
            
            # Limit diff size
            max_lines = 150
            diff_lines = diff.split('\n')
            if len(diff_lines) > max_lines:
                diff = '\n'.join(diff_lines[:max_lines]) + '\n\n... (diff truncated)'
            
            return f"{stats}\n\n{diff}"
            
        except Exception as e:
            logger.error(f"Error getting staged diff: {e}")
            return None
    
    def get_staged_files(self, repo_path: str) -> list:
        """Get list of staged files"""
        from backend.config import http_timeout_short
        try:
            result = subprocess.run(
                ["git", "diff", "--cached", "--name-only"],
                cwd=repo_path,
                capture_output=True,
                text=True,
                timeout=http_timeout_short()
            )
            
            if result.returncode == 0:
                return [f.strip() for f in result.stdout.split('\n') if f.strip()]
            
            return []
        except Exception:
            return []
    
    def get_git_context(self, repo_path: str) -> Optional[str]:
        """Get enhanced git context using git-sage utilities"""
        if not HAS_GIT_SAGE:
            return None

        try:
            ops = GitOperations(cwd=repo_path)
            pr = PRFinder(cwd=repo_path)

            context_parts = []

            # Get current branch info
            current_branch = ops.get_current_branch()
            if current_branch and current_branch != "HEAD":
                context_parts.append(f"Branch: {current_branch}")

            # Get PR metadata
            metadata = pr.suggest_pr_metadata()
            if metadata.get("issue_number"):
                context_parts.append(f"Issue/PR: #{metadata['issue_number']}")

            # Get recent commits for context
            try:
                recent = ops.get_commit_log(n=3)
                if recent:
                    context_parts.append("Recent commits:")
                    for commit in recent:
                        context_parts.append(f"  - {commit['sha']}: {commit['message'][:50]}")
            except Exception as e:
                logger.debug(f"Error getting recent commits: {e}")

            # Get diff stats
            try:
                stats = pr.get_diff_stats()
                if stats.get('files'):
                    context_parts.append(f"Changes: {stats['files']} files, +{stats['additions']} -{stats['deletions']}")
            except Exception as e:
                logger.debug(f"Error getting diff stats: {e}")

            return "\n".join(context_parts) if context_parts else None
        except Exception as e:
            logger.debug(f"Error getting git context: {e}")
            return None

    def analyze_changes_in_plain_language(self, repo_path: str, files: list) -> str:
        """Analyze staged changes and describe them in plain language"""
        try:
            descriptions = []
            
            from backend.config import http_timeout_short
            for file_path in files[:10]:  # Limit to first 10 files
                # Get diff for this specific file
                result = subprocess.run(
                    ["git", "diff", "--cached", "--unified=3", file_path],
                    cwd=repo_path,
                    capture_output=True,
                    text=True,
                    timeout=http_timeout_short()
                )
                
                if result.returncode != 0:
                    continue
                
                diff_content = result.stdout
                
                # Analyze the diff to understand what changed
                lines_added = diff_content.count('\n+') - diff_content.count('\n+++')
                lines_removed = diff_content.count('\n-') - diff_content.count('\n---')
                
                # Look for function/class definitions being added or modified
                added_functions = []
                modified_functions = []
                removed_functions = []
                
                # Simple heuristics to detect what changed
                if 'def ' in diff_content or 'function ' in diff_content:
                    # Extract function names being added
                    for line in diff_content.split('\n'):
                        if line.startswith('+') and ('def ' in line or 'function ' in line):
                            func_match = None
                            if 'def ' in line:
                                func_match = line.split('def ')[1].split('(')[0].strip()
                            elif 'function ' in line:
                                func_match = line.split('function ')[1].split('(')[0].strip()
                            if func_match and func_match not in added_functions:
                                added_functions.append(func_match)
                
                # Build natural language description
                file_desc_parts = []
                
                # Analyze what actually changed by looking at diff context
                context_lines = []
                for line in diff_content.split('\n')[:50]:  # Look at first 50 lines
                    if line.startswith('+') and not line.startswith('+++'):
                        context_lines.append(line[1:].strip())
                
                # Try to understand the change better
                change_type = None
                if any('def ' in line or 'function ' in line for line in context_lines):
                    change_type = "functions"
                elif any('class ' in line for line in context_lines):
                    change_type = "classes"
                elif any('import ' in line for line in context_lines):
                    change_type = "imports"
                elif any('if ' in line or 'for ' in line or 'while ' in line for line in context_lines):
                    change_type = "logic"
                
                if lines_added > 0 and lines_removed == 0:
                    # New code added
                    if added_functions:
                        funcs_str = ', '.join(added_functions[:2])
                        if len(added_functions) > 2:
                            funcs_str += f" and {len(added_functions) - 2} more"
                        file_desc_parts.append(f"added {funcs_str}")
                    elif change_type:
                        file_desc_parts.append(f"added new {change_type} ({lines_added} lines)")
                    else:
                        file_desc_parts.append(f"added {lines_added} lines of new code")
                elif lines_removed > 0 and lines_added == 0:
                    # Code removed
                    file_desc_parts.append(f"removed {lines_removed} lines")
                elif lines_added > 0 and lines_removed > 0:
                    # Modified
                    if lines_added > lines_removed * 2:
                        file_desc_parts.append(f"significantly expanded the code (added {lines_added} lines, removed {lines_removed})")
                    elif lines_removed > lines_added * 2:
                        file_desc_parts.append(f"removed code and simplified (removed {lines_removed} lines, added {lines_added})")
                    else:
                        file_desc_parts.append(f"modified existing code (added {lines_added} lines, removed {lines_removed})")
                
                # Add context about what the file does
                file_name = file_path.split('/')[-1]
                if file_name.endswith('.py'):
                    file_type = "Python script"
                elif file_name.endswith('.sh'):
                    file_type = "shell script"
                elif file_name.endswith('.go'):
                    file_type = "Go file"
                elif file_name.endswith('.md'):
                    file_type = "documentation"
                else:
                    file_type = "file"
                
                if file_desc_parts:
                    desc = f"In {file_path}: {', '.join(file_desc_parts)}"
                    descriptions.append(desc)
            
            if len(files) > 10:
                descriptions.append(f"... and {len(files) - 10} more files were modified")
            
            return "\n".join(descriptions) if descriptions else "Code changes were made to the staged files."
            
        except Exception as e:
            logger.warning(f"Error analyzing changes: {e}")
            return "Code changes were made to the staged files."
    
    def enhance_message_with_ai(self, original_message: str, diff: str, files: list, repo_path: str = None) -> str:
        """Use AI to enhance/generate commit message"""
        try:
            if repo_path is None:
                repo_path = os.getcwd()

            # Get enhanced git context (branch, PR, recent commits, diff stats)
            git_context = self.get_git_context(repo_path)

            # Get plain language description of changes
            plain_changes = self.analyze_changes_in_plain_language(repo_path, files)
            
            # Check if original message is meaningful or just placeholder
            is_placeholder = (
                not original_message or 
                original_message.strip() in ["", "temp", "wip", "update", "fix", "test"] or
                len(original_message.strip()) < 5
            )
            
            if is_placeholder:
                prompt = f"""You are writing a git commit message. Analyze the code changes and write a clear, descriptive commit message.

                {f"Git Context:\n{git_context}\n" if git_context else ""}
                What Changed:
                {plain_changes}

                REQUIREMENTS:
                1. First line: Concrete summary naming WHAT changed (under 72 chars). Be specific - e.g. "Fix path resolution in devtrack-git wrapper" not "Fix bug".
                2. Blank line
                3. Body (2-3 sentences): Briefly explain WHAT was changed and WHY. Include the key files/areas affected and the benefit.

                GOOD EXAMPLE:
                Replace ~/.devtrack fallbacks with project-root path resolution

                All backend modules now resolve paths from PROJECT_ROOT/Data instead of
                hardcoded ~/.devtrack. Fixes path mismatches when running from different
                directories and keeps config centralized in .env.

                BAD EXAMPLE (DO NOT DO THIS):
                Update files

                Changed some code.

                Write ONLY the commit message. No meta-commentary, no options, nothing else."""
            else:
                # Check if this looks like an already-enhanced message (has body paragraph)
                has_body = '\n\n' in original_message.strip() or original_message.count('\n') >= 2
                
                # Get plain language description (repo_path already available from outer scope)
                if repo_path is None:
                    repo_path = os.getcwd()
                plain_changes = self.analyze_changes_in_plain_language(repo_path, files)
                
                if has_body:
                    # Improve existing enhanced message - refine and enhance further
                    prompt = f"""You are refining an existing git commit message to make it better. Keep the good parts but improve clarity, add missing details, or enhance the explanation.

                {f"Git Context:\n{git_context}\n" if git_context else ""}
                Current Message:
                {original_message}

                What Changed:
                {plain_changes}

                REQUIREMENTS:
                1. First line: Improved summary (under 72 chars). Keep it specific and concrete.
                2. Blank line
                3. Body (2-3 sentences): Enhance the explanation - add missing context, clarify WHY, or improve wording. Keep what's good, improve what's not.

                Make the message MORE descriptive and informative than the current version.
                Write ONLY the improved commit message. No meta-commentary."""
                else:
                    # Improve a basic message - make it more descriptive
                    prompt = f"""You are improving a git commit message. Analyze the code changes and rewrite to be more descriptive.

                {f"Git Context:\n{git_context}\n" if git_context else ""}
                Original Message: {original_message}

                What Changed:
                {plain_changes}

                REQUIREMENTS:
                1. First line: Concrete summary of WHAT changed (under 72 chars). Name the specific change - e.g. "Add --dry-run to devtrack git commit" not "Improve commit".
                2. Blank line
                3. Body (2-3 sentences): Explain WHAT was changed and WHY. Reference key files or areas. Keep it concise.

                GOOD EXAMPLE:
                Add --dry-run flag to devtrack git commit

                Allows previewing the AI-enhanced commit message without committing.
                Useful for verifying enhancement before pushing. Wrapper now accepts
                --dry-run and skips git commit when present.

                BAD EXAMPLE (DO NOT DO THIS):
                Fixed bug

                Made some changes.

                Write ONLY the improved commit message. No meta-commentary, nothing else."""

            from backend.llm.base import LLMOptions
            from backend.config import http_timeout, commit_llm_temperature, commit_llm_max_tokens
            enhanced = self._get_provider().generate(
                prompt=prompt,
                options=LLMOptions(
                    temperature=commit_llm_temperature(),
                    max_tokens=commit_llm_max_tokens(),
                ),
                timeout=http_timeout(),
            )

            if not enhanced:
                return original_message
            enhanced = enhanced.replace("```", "").strip()
            
            # Remove common meta-commentary patterns
            meta_patterns = [
                "Here's", "here's", "Okay,", "Option ", "OPTION ",
                "I've aimed", "Choose the", "This is", "Let me"
            ]
            for pattern in meta_patterns:
                if enhanced.startswith(pattern):
                    # Find first newline and take everything after
                    lines = enhanced.split('\n')
                    for i, line in enumerate(lines):
                        if not any(p in line for p in meta_patterns):
                            enhanced = '\n'.join(lines[i:])
                            break
            
            # If AI response is empty or looks wrong, return original
            if not enhanced or len(enhanced) < 5 or enhanced.startswith("I "):
                return original_message
            
            return enhanced
            
        except Exception as e:
            logger.error(f"Error enhancing message: {e}")
            return original_message
    
    def process_commit_message(self, commit_msg_file: str, repo_path: str) -> bool:
        """
        Main entry point: enhance the commit message file
        
        Args:
            commit_msg_file: Path to the COMMIT_EDITMSG file
            repo_path: Path to the git repository
            
        Returns:
            True if message was enhanced, False otherwise
        """
        try:
            # Read original message
            with open(commit_msg_file, 'r') as f:
                original = f.read()
            
            # Remove comment lines
            lines = original.split('\n')
            message_lines = [l for l in lines if not l.startswith('#')]
            original_message = '\n'.join(message_lines).strip()
            
            # Get staged changes
            diff = self.get_staged_diff(repo_path)
            if not diff:
                logger.warning("No staged changes found")
                return False
            
            files = self.get_staged_files(repo_path)
            if not files:
                logger.warning("No staged files found")
                return False
            
            logger.info(f"Enhancing commit message with AI ({len(files)} files changed)...")
            
            # Enhance with AI
            enhanced = self.enhance_message_with_ai(original_message, diff, files, repo_path)
            
            if enhanced == original_message:
                logger.info("No enhancement needed or AI unavailable")
                return False
            
            # Write enhanced message back (clean, no metadata)
            with open(commit_msg_file, 'w') as f:
                f.write(enhanced)
                # Add back the git comments
                comment_lines = [l for l in lines if l.startswith('#')]
                if comment_lines:
                    f.write('\n\n')
                    f.write('\n'.join(comment_lines))
                    f.write('\n')
            
            logger.info("✓ Commit message enhanced")
            return True
            
        except Exception as e:
            logger.error(f"Error processing commit message: {e}")
            return False


def main():
    """CLI entry point for git hook"""
    if len(sys.argv) < 2:
        print("Usage: commit_message_enhancer.py <commit-msg-file> [commit-source] [commit-sha]")
        sys.exit(1)
    
    commit_msg_file = sys.argv[1]
    commit_source = sys.argv[2] if len(sys.argv) > 2 else None
    
    # Don't enhance if this is a merge or squash (but DO enhance for 'message', 'commit', etc.)
    if commit_source in ['merge', 'squash']:
        sys.exit(0)
    
    # Get repo path from environment variable (set by git hook) or use cwd
    repo_path = os.getenv('GIT_DIR')
    if repo_path and repo_path.endswith('.git'):
        repo_path = os.path.dirname(repo_path)
    if not repo_path or not os.path.exists(repo_path):
        repo_path = os.getcwd()
    
    # Try to find .git directory in parents if not at repo root
    check_path = repo_path
    for _ in range(5):  # Check up to 5 levels up
        if os.path.exists(os.path.join(check_path, '.git')):
            repo_path = check_path
            break
        parent = os.path.dirname(check_path)
        if parent == check_path:  # Reached root
            break
        check_path = parent
    
    logger.info(f"Working in repository: {repo_path}")
    logger.info(f"Commit source: {commit_source or 'manual'}")
    
    # Load env from project root
    try:
        sys.path.insert(0, str(Path(__file__).resolve().parent.parent))
        from backend.config import _load_env
        _load_env()
    except ImportError:
        try:
            from dotenv import load_dotenv
            # Walk up from script to find project root .env
            cur = Path(__file__).resolve().parent.parent
            for _ in range(5):
                env_path = cur / ".env"
                if env_path.exists():
                    load_dotenv(env_path)
                    break
                if (cur / ".env_sample").exists():
                    load_dotenv(cur / ".env_sample")
                    break
                parent = cur.parent
                if parent == cur:
                    break
                cur = parent
        except ImportError:
            pass

    enhancer = CommitMessageEnhancer()
    success = enhancer.process_commit_message(commit_msg_file, repo_path)

    # Print to stdout so wrapper script can detect success
    if success:
        print("enhanced", file=sys.stdout)
        sys.stdout.flush()

    sys.exit(0)


if __name__ == "__main__":
    main()
