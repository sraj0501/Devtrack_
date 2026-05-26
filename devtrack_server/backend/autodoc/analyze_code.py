"""
End-to-end demo with optional Ollama LLM call.

Usage:
  python analyze_with_ollama.py path/to/file.py            # analyze and print summary (no LLM)
  python analyze_with_ollama.py path/to/file.py --ollama   # analyze, call Ollama, print LLM result

Environment:
  OLLAMA_MODEL (optional) - Ollama model name, default: "llama3.1:8b"
  OLLAMA_URL (optional) - Ollama server base URL, default: http://localhost:11434

Output:
  - Deterministic summary_md printed to stdout
  - If --ollama used: also prints the model's Markdown output

Notes:
  - This is a minimal, pragmatic demo for local use. It performs static AST analysis,
    best-effort cross-file resolution (short-name), and builds a factual prompt
    that is sent to Ollama for a polished Code Flow section.
"""

import ast
import os
import sys
import json
import argparse
import httpx
from typing import Dict, List, Tuple
from dotenv import load_dotenv

load_dotenv(".env")

# -----------------------------
# Analyzer
# -----------------------------

def analyze_file_calls(file_path: str):
    """Return defs, calls_by_scope, unused_functions."""
    with open(file_path, "r", encoding="utf-8") as f:
        src = f.read()
    tree = ast.parse(src, filename=file_path)

    defs: Dict[str, Dict] = {}
    scope_stack: List[str] = []

    class DefCollector(ast.NodeVisitor):
        def visit_ClassDef(self, node: ast.ClassDef):
            scope_stack.append(node.name)
            self.generic_visit(node)
            scope_stack.pop()

        def visit_FunctionDef(self, node: ast.FunctionDef):
            fq = ".".join(scope_stack + [node.name]) if scope_stack else node.name
            defs[fq] = {"short_name": node.name, "lineno": node.lineno, "end_lineno": getattr(node, "end_lineno", None)}
            scope_stack.append(node.name)
            self.generic_visit(node)
            scope_stack.pop()

        def visit_AsyncFunctionDef(self, node: ast.AsyncFunctionDef):
            fq = ".".join(scope_stack + [node.name]) if scope_stack else node.name
            defs[fq] = {"short_name": node.name, "lineno": node.lineno, "end_lineno": getattr(node, "end_lineno", None)}
            scope_stack.append(node.name)
            self.generic_visit(node)
            scope_stack.pop()

        def visit_Assign(self, node: ast.Assign):
            if isinstance(node.value, ast.Lambda):
                for t in node.targets:
                    if isinstance(t, ast.Name):
                        fq = ".".join(scope_stack + [t.id]) if scope_stack else t.id
                        defs[fq] = {"short_name": t.id, "lineno": node.lineno, "end_lineno": getattr(node, "end_lineno", None)}
            self.generic_visit(node)

    DefCollector().visit(tree)

    calls_by_scope: Dict[str, set] = {}
    current_scope_stack: List[str] = []

    def scope_name():
        return ".".join(current_scope_stack) if current_scope_stack else "__module__"

    class CallCollector(ast.NodeVisitor):
        def visit_ClassDef(self, node: ast.ClassDef):
            current_scope_stack.append(node.name)
            self.generic_visit(node)
            current_scope_stack.pop()

        def visit_FunctionDef(self, node: ast.FunctionDef):
            current_scope_stack.append(node.name)
            self.generic_visit(node)
            current_scope_stack.pop()

        def visit_AsyncFunctionDef(self, node: ast.AsyncFunctionDef):
            current_scope_stack.append(node.name)
            self.generic_visit(node)
            current_scope_stack.pop()

        def visit_Call(self, node: ast.Call):
            variants = set()
            func = node.func
            if isinstance(func, ast.Name):
                variants.add(func.id)
            elif isinstance(func, ast.Attribute):
                variants.add(func.attr)
                base = func.value
                if isinstance(base, ast.Name):
                    variants.add(f"{base.id}.{func.attr}")
                # collect dotted chain best-effort
                names = []
                cur = func
                while isinstance(cur, ast.Attribute):
                    names.append(cur.attr)
                    cur = cur.value
                if isinstance(cur, ast.Name):
                    names.append(cur.id)
                    variants.add(".".join(reversed(names)))
            if not variants:
                variants.add("<unknown>")
            sc = scope_name()
            calls_by_scope.setdefault(sc, set()).update(variants)
            self.generic_visit(node)

    CallCollector().visit(tree)

    called_short_names = set()
    for vs in calls_by_scope.values():
        for v in vs:
            called_short_names.add(v.split(".")[-1])
    unused = [fq for fq, info in defs.items() if (info["short_name"] not in called_short_names) and (not
        fq.__contains__("__init__")
    )]

    return {"defs": defs, "calls_by_scope": calls_by_scope, "unused_functions": unused}


# -----------------------------
# Cross-file/index resolver (best-effort)
# -----------------------------

def index_repo_defs(root_dir: str) -> Dict[str, List[Tuple[str, int]]]:
    defs_index: Dict[str, List[Tuple[str, int]]] = {}
    for dirpath, _, filenames in os.walk(root_dir):
        for fn in filenames:
            if not fn.endswith(".py"):
                continue
            path = os.path.join(dirpath, fn)
            try:
                with open(path, "r", encoding="utf-8") as f:
                    tree = ast.parse(f.read(), filename=path)
            except Exception:
                continue
            for node in ast.walk(tree):
                if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
                    defs_index.setdefault(node.name, []).append((path, getattr(node, "lineno", 0)))
    return defs_index


def resolve_callee(callee: str, defs_index: Dict[str, List[Tuple[str, int]]]) -> List[Tuple[str, int]]:
    short = callee.split(".")[-1]
    return defs_index.get(short, [])


# -----------------------------
# Module-level sequence + prompt builder
# -----------------------------

def extract_module_call_sequence(file_path: str):
    with open(file_path, "r", encoding="utf-8") as f:
        src = f.read()
    tree = ast.parse(src, filename=file_path)
    seq = []

    class TopLevelVisitor(ast.NodeVisitor):
        def visit_Expr(self, node: ast.Expr):
            if isinstance(node.value, ast.Call):
                func = node.value.func
                if isinstance(func, ast.Name):
                    seq.append({"repr": func.id, "lineno": node.lineno})
                elif isinstance(func, ast.Attribute):
                    base = getattr(func.value, "id", None)
                    seq.append({"repr": f"{base}.{func.attr}" if base else func.attr, "lineno": node.lineno})
            self.generic_visit(node)

        def visit_Assign(self, node: ast.Assign):
            if isinstance(node.value, ast.Call):
                func = node.value.func
                if isinstance(func, ast.Name):
                    seq.append({"repr": func.id, "lineno": node.lineno})
            self.generic_visit(node)

    TopLevelVisitor().visit(tree)
    seq.sort(key=lambda x: x["lineno"])
    return seq


def make_codeflow_and_prompt(file_path: str, analysis: Dict) -> Dict:
    defs = analysis.get("defs", {})
    calls_by_scope = analysis.get("calls_by_scope", {})
    unused = analysis.get("unused_functions", [])
    module_seq = extract_module_call_sequence(file_path)

    bullets: List[str] = []
    if module_seq:
        bullets.append("When executed as a script, the module performs these steps:")
        for i in module_seq:
            bullets.append(f"- Line {i['lineno']}: `{i['repr']}` is invoked.")
    else:
        bullets.append("No module-level invocations detected.")

    for fq, info in defs.items():
        calls = calls_by_scope.get(fq, set())
        if calls:
            bullets.append(f"- `{fq}` (line {info['lineno']}) calls: {', '.join(sorted(calls))}.")
        else:
            bullets.append(f"- `{fq}` (line {info['lineno']}) does not call other functions.")

    if unused:
        bullets.append("Unused functions:")
        for u in unused:
            bullets.append(f"- `{u}`")

    summary_md = "## Code flow (auto-generated)\n\n" + "\n".join(bullets)

    facts_lines = []
    facts_lines.append(f"FILE: {file_path}")
    facts_lines.append("MODULE_SEQUENCE:")
    for item in module_seq:
        facts_lines.append(f"- {item['lineno']}: {item['repr']}")
    facts_lines.append("DEFINITIONS_AND_CALLEES:")
    for fq, info in defs.items():
        vn = calls_by_scope.get(fq, set())
        facts_lines.append(f"- {fq} (line {info.get('lineno')}): calls -> {sorted(list(vn))}")
    facts_lines.append("UNUSED_FUNCTIONS: " + (", ".join(unused) if unused else "none"))

    llm_prompt = (
        "SYSTEM: You are a factual code-documentation assistant. Use only the facts provided below. "
        "Output Markdown with a short Code Flow section (3–7 bullets). Do not invent behavior.\n\n"
        "FACTS:\n" + "\n".join(facts_lines) + "\n\nTASK:\nProduce a concise Code Flow section describing runtime order and relationships. "
        "Explicitly list any defined but unused functions.\n\nOUTPUT: Markdown"
    )

    return {"summary_md": summary_md, "llm_prompt": llm_prompt}


# -----------------------------
# Ollama caller
# -----------------------------

def call_ollama(prompt: str, model: str, ollama_url: str | None , timeout: int = 60) -> str:
    from backend.config import ollama_host, ollama_model
    model = model or ollama_model()
    base = ollama_url or ollama_host()
    url = base.rstrip("/") + "/api/generate"
    payload = {
        "model": model,
        "prompt": prompt,
        "temperature": 0.0,
        "max_new_tokens": 512,
        "stream": False,
    }
    try:
        with httpx.Client(timeout=timeout) as client:
            r = client.post(url, json=payload)
            r.raise_for_status()
            data = r.json()
            # Common shapes: {"response": "..."} or {"choices": [{"text": "..."}]}
            if isinstance(data, dict):
                if "response" in data:
                    return data["response"]
                if "choices" in data and data["choices"]:
                    ch = data["choices"][0]
                    return ch.get("text") or ch.get("message") or json.dumps(ch)
            return json.dumps(data)
    except Exception as e:
        return f"<OLLAMA_CALL_FAILED: {e}>"


# -----------------------------
# CLI / main
# -----------------------------

def main():
    parser = argparse.ArgumentParser(description="Analyze Python file and optionally call Ollama for code flow docs")
    parser.add_argument("file", help="Python file to analyze")
    parser.add_argument("--ollama", action="store_true", help="Call local Ollama with the generated factual prompt")
    parser.add_argument("--repo-root", default=".", help="Root to index for cross-file resolution (default: current dir)")
    args = parser.parse_args()

    if not os.path.exists(args.file):
        print(f"File not found: {args.file}")
        sys.exit(2)

    analysis = analyze_file_calls(args.file)
    defs_index = index_repo_defs(args.repo_root)

    # best-effort resolution: attach resolved locations to analysis (not modifying original structure deeply)
    resolved_map = {}
    for scope, callees in analysis["calls_by_scope"].items():
        resolved_map[scope] = {}
        for c in sorted(callees):
            resolved = resolve_callee(c, defs_index)
            resolved_map[scope][c] = resolved

    out = make_codeflow_and_prompt(args.file, analysis)

    # Print deterministic summary
    print(out["summary_md"])

    # Optionally call Ollama
    if args.ollama:
        print("\n--- Calling Ollama with factual prompt (temperature=0.0) ---\n")
        from backend.config import ollama_host
        llm_out = call_ollama(out["llm_prompt"], model="mistral:latest", ollama_url=ollama_host())
        print(llm_out)

if __name__ == "__main__":
    main()
