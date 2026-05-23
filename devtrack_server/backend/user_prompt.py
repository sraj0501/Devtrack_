"""
Terminal User Interface (TUI) for DevTrack

This module provides a simple but effective terminal interface for:
- Prompting users for work updates
- Displaying parsed task information
- Showing enhanced descriptions
- Confirming task updates

Works in both interactive terminals and non-interactive environments.
"""

import os
import sys
import logging
import select
from typing import Optional, Dict, List, Tuple, Any
from dataclasses import dataclass
from datetime import datetime

logger = logging.getLogger(__name__)


# ANSI color codes
class Colors:
    """ANSI color codes for terminal output"""
    RESET = "\033[0m"
    BOLD = "\033[1m"
    DIM = "\033[2m"
    UNDERLINE = "\033[4m"
    
    # Colors
    RED = "\033[31m"
    GREEN = "\033[32m"
    YELLOW = "\033[33m"
    BLUE = "\033[34m"
    MAGENTA = "\033[35m"
    CYAN = "\033[36m"
    WHITE = "\033[37m"
    
    # Bright colors
    BRIGHT_RED = "\033[91m"
    BRIGHT_GREEN = "\033[92m"
    BRIGHT_YELLOW = "\033[93m"
    BRIGHT_BLUE = "\033[94m"
    BRIGHT_MAGENTA = "\033[95m"
    BRIGHT_CYAN = "\033[96m"
    
    # Background
    BG_BLUE = "\033[44m"
    BG_GREEN = "\033[42m"
    
    @classmethod
    def disable(cls):
        """Disable colors (for non-terminal output)"""
        for attr in dir(cls):
            if not attr.startswith('_') and attr.isupper():
                setattr(cls, attr, "")


# Check if we should use colors
if not sys.stdout.isatty():
    Colors.disable()


@dataclass
class UserResponse:
    """Represents a user's response to a prompt"""
    raw_input: str
    timestamp: datetime
    timed_out: bool = False
    cancelled: bool = False
    
    def is_valid(self) -> bool:
        """Check if response is valid (not empty, not timed out, not cancelled)"""
        return bool(self.raw_input.strip()) and not self.timed_out and not self.cancelled


class DevTrackTUI:
    """Terminal User Interface for DevTrack"""
    
    # Box drawing characters
    BOX_TL = "┌"
    BOX_TR = "┐"
    BOX_BL = "└"
    BOX_BR = "┘"
    BOX_H = "─"
    BOX_V = "│"
    
    # Icons
    ICON_CLOCK = "⏰"
    ICON_CHECK = "✓"
    ICON_CROSS = "✗"
    ICON_ARROW = "➜"
    ICON_STAR = "★"
    ICON_TASK = "📋"
    ICON_BRAIN = "🧠"
    ICON_ROCKET = "🚀"
    
    def __init__(self, width: int = 60):
        """
        Initialize TUI
        
        Args:
            width: Default width for boxes and formatting
        """
        self.width = width
        self.interactive = sys.stdin.isatty()
    
    def clear_line(self):
        """Clear the current line"""
        sys.stdout.write('\r' + ' ' * self.width + '\r')
        sys.stdout.flush()
    
    def print_header(self, title: str, icon: str = "🚀"):
        """Print a styled header"""
        line = self.BOX_H * (self.width - 2)
        print(f"\n{Colors.CYAN}{self.BOX_TL}{line}{self.BOX_TR}{Colors.RESET}")
        
        # Center the title
        padded_title = f" {icon} {title} "
        padding = (self.width - len(padded_title)) // 2
        print(f"{Colors.CYAN}{self.BOX_V}{Colors.RESET}{' ' * padding}{Colors.BOLD}{padded_title}{Colors.RESET}{' ' * (self.width - padding - len(padded_title) - 2)}{Colors.CYAN}{self.BOX_V}{Colors.RESET}")
        
        print(f"{Colors.CYAN}{self.BOX_BL}{line}{self.BOX_BR}{Colors.RESET}")
    
    def print_box(self, lines: List[str], title: Optional[str] = None, 
                  color: str = Colors.WHITE):
        """Print content in a box"""
        border_h = self.BOX_H * (self.width - 2)
        
        # Top border
        if title:
            title_str = f" {title} "
            padding = (self.width - 2 - len(title_str)) // 2
            top = f"{self.BOX_TL}{self.BOX_H * padding}{title_str}{self.BOX_H * (self.width - 2 - padding - len(title_str))}{self.BOX_TR}"
        else:
            top = f"{self.BOX_TL}{border_h}{self.BOX_TR}"
        
        print(f"{color}{top}{Colors.RESET}")
        
        # Content
        for line in lines:
            # Truncate if too long
            if len(line) > self.width - 4:
                line = line[:self.width - 7] + "..."
            padding = self.width - 4 - len(line)
            print(f"{color}{self.BOX_V}{Colors.RESET} {line}{' ' * padding} {color}{self.BOX_V}{Colors.RESET}")
        
        # Bottom border
        print(f"{color}{self.BOX_BL}{border_h}{self.BOX_BR}{Colors.RESET}")
    
    def print_section(self, title: str, content: Dict[str, str], 
                      icon: str = "📋"):
        """Print a section with key-value pairs"""
        print(f"\n{Colors.BOLD}{icon} {title}{Colors.RESET}")
        print(f"{Colors.DIM}{'─' * 40}{Colors.RESET}")
        
        for key, value in content.items():
            if value:
                print(f"  {Colors.CYAN}{key}:{Colors.RESET} {value}")
    
    def print_success(self, message: str):
        """Print a success message"""
        print(f"\n{Colors.GREEN}{self.ICON_CHECK} {message}{Colors.RESET}")
    
    def print_error(self, message: str):
        """Print an error message"""
        print(f"\n{Colors.RED}{self.ICON_CROSS} {message}{Colors.RESET}")
    
    def print_warning(self, message: str):
        """Print a warning message"""
        print(f"\n{Colors.YELLOW}⚠ {message}{Colors.RESET}")
    
    def print_info(self, message: str):
        """Print an info message"""
        print(f"\n{Colors.BLUE}ℹ {message}{Colors.RESET}")
    
    def prompt_user(self, prompt: str, timeout: Optional[int] = None,
                    default: Optional[str] = None) -> UserResponse:
        """
        Prompt user for input
        
        Args:
            prompt: The prompt to display
            timeout: Optional timeout in seconds
            default: Default value if timeout or empty
            
        Returns:
            UserResponse object
        """
        timestamp = datetime.now()
        
        # Format prompt with colors
        prompt_text = f"\n{Colors.BRIGHT_CYAN}{self.ICON_ARROW} {prompt}{Colors.RESET}"
        if default:
            prompt_text += f" {Colors.DIM}[{default}]{Colors.RESET}"
        prompt_text += f"\n{Colors.YELLOW}>{Colors.RESET} "
        
        if not self.interactive:
            # Non-interactive mode: use DEVTRACK_INPUT env for CI/test, else default
            prefill = os.environ.get("DEVTRACK_INPUT", "").strip()
            if prefill:
                logger.info("Non-interactive: using DEVTRACK_INPUT from env")
                return UserResponse(raw_input=prefill, timestamp=timestamp)
            logger.warning("Non-interactive terminal, cannot prompt user")
            return UserResponse(
                raw_input=default or "",
                timestamp=timestamp,
                timed_out=True
            )
        
        if timeout:
            return self._prompt_with_timeout(prompt_text, timeout, default, timestamp)
        else:
            return self._prompt_simple(prompt_text, default, timestamp)
    
    def _prompt_simple(self, prompt_text: str, default: Optional[str],
                       timestamp: datetime) -> UserResponse:
        """Simple prompt without timeout"""
        try:
            print(prompt_text, end="", flush=True)
            response = input()
            
            if not response and default:
                response = default
                
            return UserResponse(
                raw_input=response,
                timestamp=timestamp
            )
        except KeyboardInterrupt:
            print()  # New line after ^C
            return UserResponse(
                raw_input="",
                timestamp=timestamp,
                cancelled=True
            )
        except EOFError:
            return UserResponse(
                raw_input=default or "",
                timestamp=timestamp,
                timed_out=True
            )
    
    def _prompt_with_timeout(self, prompt_text: str, timeout: int,
                             default: Optional[str],
                             timestamp: datetime) -> UserResponse:
        """Prompt with timeout (Unix-like systems)"""
        print(prompt_text, end="", flush=True)
        
        # Show countdown
        print(f" {Colors.DIM}({timeout}s timeout){Colors.RESET}", end="", flush=True)
        print("\b" * 20, end="", flush=True)  # Move cursor back
        
        try:
            # Use select for timeout (Unix-like systems)
            if hasattr(select, 'select'):
                rlist, _, _ = select.select([sys.stdin], [], [], timeout)
                if rlist:
                    response = sys.stdin.readline().strip()
                    if not response and default:
                        response = default
                    return UserResponse(
                        raw_input=response,
                        timestamp=timestamp
                    )
                else:
                    print()  # New line
                    self.print_warning(f"Timed out after {timeout}s")
                    return UserResponse(
                        raw_input=default or "",
                        timestamp=timestamp,
                        timed_out=True
                    )
            else:
                # Fallback for Windows - simple input without timeout
                response = input()
                if not response and default:
                    response = default
                return UserResponse(
                    raw_input=response,
                    timestamp=timestamp
                )
                
        except KeyboardInterrupt:
            print()
            return UserResponse(
                raw_input="",
                timestamp=timestamp,
                cancelled=True
            )
    
    def prompt_yes_no(self, question: str, default: bool = True) -> bool:
        """
        Prompt for yes/no confirmation
        
        Args:
            question: The question to ask
            default: Default value
            
        Returns:
            True for yes, False for no
        """
        default_str = "Y/n" if default else "y/N"

        from backend.config import prompt_timeout_simple
        prompt = f"{question} [{default_str}]"
        response = self.prompt_user(prompt, timeout=prompt_timeout_simple())
        
        if response.timed_out or response.cancelled or not response.raw_input:
            return default
        
        answer = response.raw_input.strip().lower()
        if answer in ['y', 'yes']:
            return True
        elif answer in ['n', 'no']:
            return False
        else:
            return default
    
    def prompt_choice(self, question: str, options: List[str],
                      default: int = 0) -> Tuple[int, str]:
        """
        Prompt user to choose from options
        
        Args:
            question: The question to ask
            options: List of options
            default: Default option index (0-based)
            
        Returns:
            Tuple of (selected index, selected value)
        """
        print(f"\n{Colors.BRIGHT_CYAN}{question}{Colors.RESET}")
        
        for i, option in enumerate(options):
            marker = f"{Colors.GREEN}*{Colors.RESET}" if i == default else " "
            print(f"  {marker} [{i+1}] {option}")
        
        from backend.config import prompt_timeout_work
        response = self.prompt_user(
            f"Enter choice (1-{len(options)})",
            timeout=prompt_timeout_work(),
            default=str(default + 1)
        )
        
        if response.timed_out or response.cancelled:
            return default, options[default]
        
        try:
            choice = int(response.raw_input.strip()) - 1
            if 0 <= choice < len(options):
                return choice, options[choice]
        except ValueError:
            pass
        
        return default, options[default]
    
    def display_parsed_task(self, parsed_task: Dict[str, Any]):
        """Display a parsed task nicely"""
        content = {}
        
        if parsed_task.get("project"):
            content["Project"] = parsed_task["project"]
        if parsed_task.get("ticket_id"):
            content["Ticket"] = parsed_task["ticket_id"]
        if parsed_task.get("action_verb"):
            content["Action"] = parsed_task["action_verb"]
        if parsed_task.get("status"):
            # Add color to status
            status = parsed_task["status"]
            if status in ["completed", "done"]:
                status = f"{Colors.GREEN}{status}{Colors.RESET}"
            elif status in ["in_progress", "working"]:
                status = f"{Colors.YELLOW}{status}{Colors.RESET}"
            elif status in ["blocked"]:
                status = f"{Colors.RED}{status}{Colors.RESET}"
            content["Status"] = status
        if parsed_task.get("time_spent"):
            content["Time Spent"] = parsed_task["time_spent"]
        if parsed_task.get("description"):
            content["Description"] = parsed_task["description"][:50] + "..." if len(parsed_task.get("description", "")) > 50 else parsed_task.get("description", "")
        if parsed_task.get("confidence"):
            confidence = parsed_task["confidence"]
            conf_color = Colors.GREEN if confidence > 0.7 else Colors.YELLOW if confidence > 0.4 else Colors.RED
            content["Confidence"] = f"{conf_color}{confidence:.0%}{Colors.RESET}"
        
        self.print_section("Parsed Task Information", content, icon="📋")
    
    def display_enhanced_description(self, enhanced: Dict[str, Any]):
        """Display enhanced description"""
        print(f"\n{Colors.BOLD}{self.ICON_BRAIN} AI-Enhanced Description{Colors.RESET}")
        print(f"{Colors.DIM}{'─' * 40}{Colors.RESET}")
        
        if enhanced.get("enhanced"):
            print(f"\n{Colors.WHITE}{enhanced['enhanced']}{Colors.RESET}")
        
        if enhanced.get("summary"):
            print(f"\n{Colors.DIM}Summary:{Colors.RESET} {enhanced['summary']}")
        
        if enhanced.get("category"):
            cat = enhanced["category"]
            cat_colors = {
                "feature": Colors.GREEN,
                "bugfix": Colors.RED,
                "refactor": Colors.BLUE,
                "docs": Colors.CYAN,
                "test": Colors.MAGENTA,
            }
            color = cat_colors.get(cat, Colors.WHITE)
            print(f"{Colors.DIM}Category:{Colors.RESET} {color}{cat}{Colors.RESET}")
        
        if enhanced.get("keywords"):
            keywords = ", ".join(enhanced["keywords"])
            print(f"{Colors.DIM}Keywords:{Colors.RESET} {keywords}")
    
    def show_progress(self, message: str, done: bool = False):
        """Show a progress indicator"""
        if done:
            print(f"\r{Colors.GREEN}{self.ICON_CHECK}{Colors.RESET} {message}" + " " * 20)
        else:
            print(f"\r{Colors.YELLOW}⏳{Colors.RESET} {message}...", end="", flush=True)
    
    def prompt_work_update(self, context: Optional[Dict] = None) -> UserResponse:
        """
        Main prompt for getting work update from user
        
        Args:
            context: Optional context (time since last update, etc.)
            
        Returns:
            UserResponse with the user's input
        """
        self.print_header("DevTrack - Work Update", self.ICON_CLOCK)
        
        # Show context
        if context:
            if context.get("interval_mins"):
                print(f"\n{Colors.DIM}Time since last check: {context['interval_mins']} minutes{Colors.RESET}")
            if context.get("last_commit"):
                print(f"{Colors.DIM}Last commit: {context['last_commit'][:50]}...{Colors.RESET}")
        
        # Main prompt
        print(f"\n{Colors.WHITE}What have you been working on?{Colors.RESET}")
        print(f"{Colors.DIM}(Include ticket numbers like #123 or PROJ-456, time like '2h' or '30min'){Colors.RESET}")

        from backend.config import prompt_timeout_task
        response = self.prompt_user(
            "Describe your work",
            timeout=prompt_timeout_task(),
            default=""
        )
        
        return response


# Quick function for simple prompts
def prompt_for_work_update(context: Optional[Dict] = None) -> UserResponse:
    """Quick helper to prompt for work update"""
    tui = DevTrackTUI()
    return tui.prompt_work_update(context)


# Example usage
if __name__ == "__main__":
    tui = DevTrackTUI()
    
    # Demo the TUI
    tui.print_header("DevTrack TUI Demo", "🎨")
    
    tui.print_info("This is an info message")
    tui.print_success("This is a success message")
    tui.print_warning("This is a warning message")
    tui.print_error("This is an error message")
    
    # Box demo
    tui.print_box([
        "Line 1: Some content here",
        "Line 2: More content",
        "Line 3: Even more content"
    ], title="Demo Box", color=Colors.CYAN)
    
    # Parsed task demo
    tui.display_parsed_task({
        "project": "DevTrack",
        "ticket_id": "DT-123",
        "action_verb": "fixed",
        "status": "completed",
        "time_spent": "2h",
        "description": "Fixed the login authentication bug",
        "confidence": 0.85
    })
    
    # Enhanced description demo
    tui.display_enhanced_description({
        "enhanced": "Implemented user authentication fix that resolves login timeout issues. Updated session handling and added proper error messages.",
        "summary": "Fixed login authentication bug",
        "category": "bugfix",
        "keywords": ["authentication", "login", "bugfix"]
    })
    
    # Interactive demo
    if sys.stdin.isatty():
        response = tui.prompt_work_update({
            "interval_mins": 30,
            "last_commit": "Fixed login bug #123"
        })
        
        if response.is_valid():
            tui.print_success(f"Got: {response.raw_input}")
        elif response.cancelled:
            tui.print_warning("Cancelled by user")
        else:
            tui.print_warning("No input received")
