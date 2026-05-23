#!/usr/bin/env python3
"""git-sage: Local LLM-powered git assistant with agentic execution."""
import argparse
import os
import subprocess
import sys

from .config import load_config, save_config, show_config
from .context import get_repo_context, format_context, run_git
from .llm import LLMBackend, parse_response
from .agent import GitAgent, _devtrack_git_cmd

RESET  = "\033[0m";  BOLD  = "\033[1m";   DIM   = "\033[2m"
GREEN  = "\033[92m"; YELLOW= "\033[93m";  CYAN  = "\033[96m"
RED    = "\033[91m"; MAGENTA="\033[95m"


def print_banner():
    print(f"\n{CYAN}{BOLD}🌿 git-sage{RESET}  {DIM}local LLM git co-pilot{RESET}\n")


def confirm(prompt: str) -> bool:
    while True:
        ans = input(f"{prompt} [y/n]: ").strip().lower()
        if ans in ("y", "yes"): return True
        if ans in ("n", "no"):  return False


# ── Session approval mode dialog ──────────────────────────────────────────────

def prompt_approval_mode(auto: bool = False) -> str:
    """Show a trust dialog for this session. Returns 'auto', 'review', or 'suggest'."""
    if auto:
        return "auto"

    print(f"\n{CYAN}{BOLD}Session approval mode:{RESET}")
    print(f"  {BOLD}[a]{RESET}  Accept all    — run every command without asking  {DIM}(like --yes){RESET}")
    print(f"  {BOLD}[r]{RESET}  Review each   — confirm every command before it runs")
    print(f"  {BOLD}[s]{RESET}  Suggest only  — show commands, never execute them\n")

    while True:
        choice = input(f"  {YELLOW}Choose mode{RESET} [a/r/s]: ").strip().lower()
        if choice in ("a", "auto", "accept"):
            print(f"  {GREEN}✓ Auto-accept mode — all commands will run automatically.{RESET}\n")
            return "auto"
        if choice in ("r", "review"):
            print(f"  {YELLOW}✓ Review mode — you'll confirm each command.{RESET}\n")
            return "review"
        if choice in ("s", "suggest"):
            print(f"  {DIM}✓ Suggest mode — commands will be shown but not run.{RESET}\n")
            return "suggest"
        print(f"  {RED}Please enter a, r, or s.{RESET}")


# ── Session history display ───────────────────────────────────────────────────

def show_session_history(agent: GitAgent) -> list[dict]:
    """Print numbered step history. Returns the list of executed steps."""
    steps = agent.get_step_history()
    if not steps:
        print(f"  {DIM}No commands were executed in this session.{RESET}")
        return steps

    print(f"\n{CYAN}{BOLD}Session command history:{RESET}")
    for i, step in enumerate(steps, 1):
        ok = step.get("exit_code", 0) == 0
        icon = f"{GREEN}✓{RESET}" if ok else f"{RED}✗{RESET}"
        head = step.get("git_head_before", "")
        head_str = f"{DIM}  (HEAD was {head[:8]}){RESET}" if head else ""
        print(f"  {DIM}{i:2d}.{RESET} {icon} {step['cmd']}{head_str}")
    print()
    return steps


def offer_undo(agent: GitAgent) -> None:
    """Interactively offer to undo steps from the session."""
    steps = show_session_history(agent)
    if not steps:
        return

    print(f"{DIM}Enter a step number to undo it (and all steps after it), or press Enter to skip.{RESET}")
    while True:
        raw = input(f"  {YELLOW}Undo step #{RESET} [1-{len(steps)}, or Enter to skip]: ").strip()
        if not raw:
            break
        if not raw.isdigit():
            print(f"  {RED}Please enter a number.{RESET}")
            continue
        idx = int(raw) - 1
        if idx < 0 or idx >= len(steps):
            print(f"  {RED}Out of range. Enter 1-{len(steps)}.{RESET}")
            continue

        step = steps[idx]
        head = step.get("git_head_before", "")
        if head:
            print(f"\n  {YELLOW}This will reset to {head[:8]}, undoing step {idx+1} and all after it:{RESET}")
            for j in range(idx, len(steps)):
                print(f"    {DIM}• {steps[j]['cmd']}{RESET}")

            if confirm(f"  {YELLOW}Confirm undo?{RESET}"):
                ok = agent.undo_step(idx)
                if ok:
                    # Refresh step list after undo
                    steps = agent.get_step_history()
                    if not steps:
                        print(f"  {DIM}No more steps. Exiting undo.{RESET}")
                        break
                    show_session_history(agent)
            else:
                print(f"  {DIM}Undo cancelled.{RESET}")
        else:
            # No HEAD snapshot — call undo_step which will print the warning
            agent.undo_step(idx)
        break


# ── execute_commands (ask mode) ───────────────────────────────────────────────

def execute_commands(commands: list[str], auto: bool = False):
    if not commands:
        return

    print(f"\n{BOLD}Suggested commands:{RESET}")
    for i, cmd in enumerate(commands, 1):
        print(f"  {DIM}{i}.{RESET} {GREEN}{cmd}{RESET}")
    print()

    if auto or confirm(f"{YELLOW}Run these commands?{RESET}"):
        for cmd in commands:
            cmd = _devtrack_git_cmd(cmd)
            print(f"{DIM}$ {cmd}{RESET}")
            result = subprocess.run(cmd, shell=True, capture_output=False, text=True)
            if result.returncode != 0:
                print(f"{RED}Command failed (exit {result.returncode}). Stopping.{RESET}")
                break
    else:
        print(f"{DIM}Commands not executed. Copy-paste them manually.{RESET}\n")


# ── ASK MODE (simple suggestion) ─────────────────────────────────────────────

def ask_mode(query: str, cfg: dict):
    cwd = os.getcwd()
    ctx = get_repo_context(cwd)
    context_str = format_context(ctx)
    backend = LLMBackend(**_backend_kwargs(cfg))
    print(f"{DIM}Asking {cfg['model']}...{RESET}\n")
    try:
        response = backend.ask(query, context_str)
    except ConnectionError as e:
        print(f"{RED}{e}{RESET}")
        sys.exit(1)
    explanation, commands = parse_response(response)
    print(f"{CYAN}{BOLD}sage>{RESET} {explanation}\n")
    if commands:
        execute_commands(commands, auto=cfg.get("auto_confirm", False))


MAX_FOLLOWUPS = 5


# ── AGENT DO MODE ─────────────────────────────────────────────────────────────

def do_mode(task: str, cfg: dict, auto: bool = False, verbose: bool = False):
    cwd = os.getcwd()
    _, is_git = run_git(["rev-parse", "--git-dir"], cwd)
    if not is_git:
        print(f"{RED}Not inside a git repository.{RESET}")
        sys.exit(1)

    # Show session approval dialog once for the whole do session
    mode = prompt_approval_mode(auto=auto)
    session_auto = (mode == "auto")
    suggest_only = (mode == "suggest")

    backend = LLMBackend(**_backend_kwargs(cfg))
    agent = GitAgent(
        backend=backend, cwd=cwd, verbose=verbose,
        auto=session_auto, suggest_only=suggest_only,
    )

    success = False
    try:
        success = agent.run(task)
    except KeyboardInterrupt:
        print(f"\n{RED}Agent interrupted by user.{RESET}\n")
        _finalize_do_session(agent)
        sys.exit(1)
    except ConnectionError as e:
        print(f"{RED}{e}{RESET}")
        sys.exit(1)

    # ── Follow-up loop ───────────────────────────────────────────────────────
    for i in range(MAX_FOLLOWUPS):
        remaining = MAX_FOLLOWUPS - i
        try:
            followup = input(
                f"\n{CYAN}Follow-up{RESET} {DIM}[{remaining} left — Enter to exit]:{RESET} "
            ).strip()
        except (KeyboardInterrupt, EOFError):
            print()
            break

        if not followup:
            break

        # Allow undo/history as inline commands during follow-up
        if followup.lower() == "history":
            show_session_history(agent)
            continue
        if followup.lower().startswith("undo"):
            parts = followup.split()
            if len(parts) > 1 and parts[1].isdigit():
                agent.undo_step(int(parts[1]) - 1)
                show_session_history(agent)
            else:
                offer_undo(agent)
            continue

        try:
            agent.followup(followup)
        except KeyboardInterrupt:
            print(f"\n{RED}Interrupted.{RESET}")
            break
        except ConnectionError as e:
            print(f"{RED}{e}{RESET}")
            break

    _finalize_do_session(agent)
    sys.exit(0 if success else 1)


def _finalize_do_session(agent: GitAgent) -> None:
    """Show undo prompt at the end of a do session if any steps were run."""
    if agent.get_step_history():
        offer_undo(agent)


# ── INTERACTIVE SHELL ─────────────────────────────────────────────────────────

def interactive_mode(cfg: dict):
    print_banner()
    print(f"Provider: {CYAN}{cfg['provider']}{RESET}  Model: {CYAN}{cfg['model']}{RESET}")
    print(f"{DIM}  ask <question>  — get command suggestions{RESET}")
    print(f"{DIM}  do  <task>      — agent plans + executes + recovers{RESET}")
    print(f"{DIM}  context         — show repo state{RESET}")
    print(f"{DIM}  history         — show commands run in this session{RESET}")
    print(f"{DIM}  undo [step]     — undo a step from the last agent run{RESET}")
    print(f"{DIM}  quit            — exit{RESET}\n")

    cwd = os.getcwd()
    _, is_git = run_git(["rev-parse", "--git-dir"], cwd)
    if not is_git:
        print(f"{YELLOW}Not inside a git repository.{RESET}\n")

    # Ask approval mode once for the whole interactive session
    mode = prompt_approval_mode(auto=cfg.get("auto_confirm", False))
    session_auto = (mode == "auto")
    suggest_only = (mode == "suggest")

    backend = LLMBackend(**_backend_kwargs(cfg))
    last_agent: GitAgent | None = None

    while True:
        try:
            raw_input = input(f"{BOLD}sage>{RESET} ").strip()
        except (KeyboardInterrupt, EOFError):
            print(f"\n{DIM}Bye!{RESET}")
            break

        if not raw_input:
            continue

        parts = raw_input.split(None, 1)
        cmd = parts[0].lower()
        rest = parts[1] if len(parts) > 1 else ""

        if cmd in ("quit", "exit", "q"):
            print(f"{DIM}Bye!{RESET}")
            break

        elif cmd == "context":
            ctx = get_repo_context(cwd)
            print(format_context(ctx))

        elif cmd == "history":
            if last_agent:
                show_session_history(last_agent)
            else:
                print(f"  {DIM}No agent has run yet in this session.{RESET}")

        elif cmd == "undo":
            if not last_agent:
                print(f"  {DIM}No agent has run yet in this session.{RESET}")
                continue

            steps = last_agent.get_step_history()
            if not steps:
                print(f"  {DIM}No steps to undo.{RESET}")
                continue

            # Allow `undo 3` to jump directly to step 3
            if rest.strip().isdigit():
                idx = int(rest.strip()) - 1
                last_agent.undo_step(idx)
                show_session_history(last_agent)
            else:
                offer_undo(last_agent)

        elif cmd == "do":
            if not rest:
                print(f"{YELLOW}Usage: do <task>{RESET}")
                continue

            last_agent = GitAgent(
                backend=backend, cwd=cwd,
                verbose=cfg.get("verbose", False),
                auto=session_auto, suggest_only=suggest_only,
            )
            try:
                last_agent.run(rest)
            except KeyboardInterrupt:
                print(f"\n{RED}Agent interrupted.{RESET}")
            finally:
                if last_agent.get_step_history():
                    print(f"{DIM}Tip: type 'history' to review steps, 'undo [N]' to revert.{RESET}\n")

        else:
            # ask (explicit or bare question)
            question = rest if cmd == "ask" else raw_input
            ctx = get_repo_context(cwd)
            context_str = format_context(ctx)
            print(f"{DIM}Thinking...{RESET}")
            try:
                response = backend.ask(question, context_str)
            except ConnectionError as e:
                print(f"{RED}{e}{RESET}\n")
                continue
            explanation, commands = parse_response(response)
            print(f"\n{CYAN}{BOLD}sage>{RESET} {explanation}\n")
            if commands:
                execute_commands(commands, auto=session_auto)


# ── CONFIG WIZARD ─────────────────────────────────────────────────────────────

def configure_wizard():
    cfg = load_config()
    print(f"\n{BOLD}git-sage configuration{RESET}\n")
    show_config(cfg)
    print("Providers: ollama | openai | groq | lmstudio | anthropic | custom")
    provider = input(f"Provider [{cfg['provider']}]: ").strip() or cfg["provider"]
    cfg["provider"] = provider

    try:
        from backend.config import git_sage_default_model, openai_model, groq_model
        default_models = {
            "ollama":    git_sage_default_model(),
            "openai":    openai_model(),
            "groq":      groq_model(),
            "lmstudio":  "local-model",
            "anthropic": "claude-haiku-4-5",
        }
    except (ImportError, ValueError):
        default_models = {
            "ollama": "llama3", "openai": "gpt-4o-mini",
            "groq": "llama-3.3-70b-versatile", "lmstudio": "local-model",
        }

    def_model = default_models.get(provider, cfg["model"])
    cfg["model"] = input(f"Model [{def_model}]: ").strip() or def_model
    url = input("Base URL (blank = default): ").strip()
    if url: cfg["base_url"] = url
    if provider in ("openai", "groq", "anthropic", "custom"):
        key = input("API key (blank = use env var): ").strip()
        if key: cfg["api_key"] = key
    cfg["auto_confirm"] = input("Auto-execute commands? [y/N]: ").strip().lower() in ("y", "yes")
    save_config(cfg)
    print(f"\n{GREEN}Config saved.{RESET}")


def _backend_kwargs(cfg: dict) -> dict:
    return {
        "provider": cfg.get("provider", "ollama"),
        "model":    cfg.get("model", "llama3"),
        "base_url": cfg.get("base_url"),
        "api_key":  cfg.get("api_key"),
    }


def main():
    parser = argparse.ArgumentParser(
        prog="git-sage",
        description="Local LLM-powered git assistant",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Modes:
  git-sage                          Interactive shell (with session approval + undo)
  git-sage ask "how do I..."        Simple command suggestion
  git-sage do  "squash last 3..."   Agentic: LLM plans + executes + recovers

Examples:
  git-sage do  "merge feature-auth into main, handle any conflicts"
  git-sage do  "squash my last 5 commits into one"
  git-sage do  "my last push broke prod, roll back to the previous commit"
  git-sage ask "what does git reflog do"

Session approval (shown automatically when using 'do'):
  [a] Accept all   — run every command without asking
  [r] Review each  — confirm before each command
  [s] Suggest only — show commands, never execute

Undo support:
  After any 'do' session, you can undo specific steps using the undo prompt.
  In interactive mode, type: history  or  undo [N]
        """
    )
    parser.add_argument("mode",  nargs="?", choices=["ask", "do"])
    parser.add_argument("query", nargs="*")
    parser.add_argument("--config",      action="store_true")
    parser.add_argument("--show-config", action="store_true")
    parser.add_argument("--provider")
    parser.add_argument("--model")
    parser.add_argument("--yes", "-y",   action="store_true", help="Accept all commands (auto mode)")
    parser.add_argument("--verbose",     action="store_true")

    args = parser.parse_args()
    cfg = load_config()

    if args.provider:  cfg["provider"] = args.provider
    if args.model:     cfg["model"]    = args.model
    if args.yes:       cfg["auto_confirm"] = True
    if args.verbose:   cfg["verbose"] = True

    if args.config:      configure_wizard(); return
    if args.show_config: show_config(cfg);   return

    if args.mode == "do":
        task = " ".join(args.query)
        if not task:
            print(f"{RED}Provide a task: git-sage do \"<task>\"{RESET}")
            sys.exit(1)
        do_mode(task, cfg, auto=cfg.get("auto_confirm", False), verbose=cfg.get("verbose", False))
    elif args.mode == "ask":
        query = " ".join(args.query)
        if not query:
            print(f"{RED}Provide a question: git-sage ask \"<question>\"{RESET}")
            sys.exit(1)
        ask_mode(query, cfg)
    elif args.mode is None and args.query:
        ask_mode(" ".join(args.query), cfg)
    else:
        interactive_mode(cfg)


if __name__ == "__main__":
    main()
