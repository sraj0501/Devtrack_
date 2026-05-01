#!/usr/bin/env python3
"""git-sage: Local LLM-powered git assistant with agentic execution."""
import argparse
import os
import subprocess
import sys

from .config import load_config, save_config, show_config
from .context import get_repo_context, format_context, run_git
from .llm import LLMBackend, parse_response
from .agent import GitAgent

RESET  = "\033[0m";  BOLD  = "\033[1m";   DIM   = "\033[2m"
GREEN  = "\033[92m"; YELLOW= "\033[93m";  CYAN  = "\033[96m"
RED    = "\033[91m"


def print_banner():
    print(f"\n{CYAN}{BOLD}🌿 git-sage{RESET}  {DIM}local LLM git co-pilot{RESET}\n")


def confirm(prompt: str) -> bool:
    while True:
        ans = input(f"{prompt} [y/n]: ").strip().lower()
        if ans in ("y", "yes"): return True
        if ans in ("n", "no"):  return False


def execute_commands(commands: list[str], auto: bool = False):
    if not commands:
        return

    print(f"\n{BOLD}Suggested commands:{RESET}")
    for i, cmd in enumerate(commands, 1):
        print(f"  {DIM}{i}.{RESET} {GREEN}{cmd}{RESET}")
    print()

    if auto or confirm(f"{YELLOW}Run these commands?{RESET}"):
        for cmd in commands:
            print(f"{DIM}$ {cmd}{RESET}")
            result = subprocess.run(cmd.split(), capture_output=False, text=True)
            if result.returncode != 0:
                print(f"{RED}Command failed (exit {result.returncode}). Stopping.{RESET}")
                break
    else:
        print(f"{DIM}Commands not executed. Copy-paste them manually.{RESET}\n")


# ASK MODE (simple suggestion)
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


# AGENT DO MODE
def do_mode(task: str, cfg: dict, auto: bool = False, verbose: bool = False):
    cwd = os.getcwd()
    _, is_git = run_git(["rev-parse", "--git-dir"], cwd)
    if not is_git:
        print(f"{RED}Not inside a git repository.{RESET}")
        sys.exit(1)
    backend = LLMBackend(**_backend_kwargs(cfg))
    agent = GitAgent(backend=backend, cwd=cwd, verbose=verbose, auto=auto)
    try:
        success = agent.run(task)
        sys.exit(0 if success else 1)
    except KeyboardInterrupt:
        print(f"\n{RED}Agent interrupted by user.{RESET}\n")
        sys.exit(1)
    except ConnectionError as e:
        print(f"{RED}{e}{RESET}")
        sys.exit(1)


# INTERACTIVE SHELL
def interactive_mode(cfg: dict):
    print_banner()
    print(f"Provider: {CYAN}{cfg['provider']}{RESET}  Model: {CYAN}{cfg['model']}{RESET}")
    print(f"{DIM}  ask <question>  — get command suggestions{RESET}")
    print(f"{DIM}  do  <task>      — agent plans + executes + recovers{RESET}")
    print(f"{DIM}  context         — show repo state{RESET}")
    print(f"{DIM}  quit            — exit{RESET}\n")

    cwd = os.getcwd()
    _, is_git = run_git(["rev-parse", "--git-dir"], cwd)
    if not is_git:
        print(f"{YELLOW}Not inside a git repository.{RESET}\n")

    backend = LLMBackend(**_backend_kwargs(cfg))

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

        elif cmd == "do":
            if not rest:
                print(f"{YELLOW}Usage: do <task>{RESET}")
                continue
            agent = GitAgent(backend=backend, cwd=cwd,
                             verbose=cfg.get("verbose", False),
                             auto=cfg.get("auto_confirm", False))
            try:
                agent.run(rest)
            except KeyboardInterrupt:
                print(f"\n{RED}Agent interrupted.{RESET}")

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
                execute_commands(commands, auto=cfg.get("auto_confirm", False))


# CONFIG WIZARD
def configure_wizard():
    cfg = load_config()
    print(f"\n{BOLD}git-sage configuration{RESET}\n")
    show_config(cfg)
    print("Providers: ollama | openai | lmstudio | custom")
    provider = input(f"Provider [{cfg['provider']}]: ").strip() or cfg["provider"]
    cfg["provider"] = provider

    # Load model defaults from config (NO HARDCODED DEFAULTS)
    try:
        from backend.config import git_sage_default_model, openai_model
        default_models = {
            "ollama": git_sage_default_model(),
            "openai": openai_model(),
            "lmstudio": "local-model"
        }
    except (ImportError, ValueError):
        # Fallback if config vars missing
        default_models = {"ollama": "llama3", "openai": "gpt-4o-mini", "lmstudio": "local-model"}

    def_model = default_models.get(provider, cfg["model"])
    cfg["model"] = input(f"Model [{def_model}]: ").strip() or def_model
    url = input("Base URL (blank = default): ").strip()
    if url: cfg["base_url"] = url
    if provider in ("openai", "custom"):
        key = input("API key (blank = skip): ").strip()
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
  git-sage                          Interactive shell
  git-sage ask "how do I..."        Simple command suggestion
  git-sage do  "squash last 3..."   Agentic: LLM plans + executes + recovers

Examples:
  git-sage do  "merge feature-auth into main, handle any conflicts"
  git-sage do  "squash my last 5 commits into one"
  git-sage do  "my last push broke prod, roll back to the previous commit"
  git-sage ask "what does git reflog do"
        """
    )
    parser.add_argument("mode",  nargs="?", choices=["ask", "do"])
    parser.add_argument("query", nargs="*")
    parser.add_argument("--config",      action="store_true")
    parser.add_argument("--show-config", action="store_true")
    parser.add_argument("--provider")
    parser.add_argument("--model")
    parser.add_argument("--yes", "-y",   action="store_true")
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
