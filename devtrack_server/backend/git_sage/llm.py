"""LLM backend supporting Ollama and OpenAI-compatible APIs."""
import json
import urllib.request
import urllib.error
from typing import Optional


# ── Simple one-shot system prompt (used by basic mode) ───────────────────────
SIMPLE_SYSTEM_PROMPT = """You are git-sage, an expert git assistant in the user's terminal.

Respond EXACTLY in this format:
---EXPLANATION---
<1-3 sentences explaining what will happen>
---COMMANDS---
<one git command per line, no bullets, no backticks — or NONE>
---END---
"""


class LLMBackend:
    def __init__(
        self,
        provider: str = "ollama",
        model: str = "llama3",
        base_url: Optional[str] = None,
        api_key: Optional[str] = None,
    ):
        self.provider = provider
        self.model = model
        self.api_key = api_key

        # Load endpoints from config (NO HARDCODED DEFAULTS)
        from backend.config import ollama_host, lmstudio_host
        defaults = {
            "ollama":   ollama_host(),
            "openai":   "https://api.openai.com/v1",
            "lmstudio": lmstudio_host(),
        }
        self.base_url = base_url or defaults.get(provider, ollama_host())

    # ── Public methods ────────────────────────────────────────────────────────

    def ask(self, user_message: str, context: str) -> str:
        """Simple one-shot ask (used by non-agent mode)."""
        full = f"{context}\n\nUser request: {user_message}"
        messages = [{"role": "user", "content": full}]
        return self.raw_chat(messages, SIMPLE_SYSTEM_PROMPT)

    def raw_chat(self, messages: list[dict], system_prompt: str) -> str:
        """Send a full conversation history and return the assistant reply."""
        if self.provider == "ollama":
            return self._ollama_chat(messages, system_prompt)
        else:
            return self._openai_chat(messages, system_prompt)

    # ── Provider implementations ──────────────────────────────────────────────

    def _ollama_chat(self, messages: list[dict], system: str) -> str:
        url = f"{self.base_url}/api/chat"
        full_messages = [{"role": "system", "content": system}] + messages
        payload = {"model": self.model, "messages": full_messages, "stream": False}
        body = self._post(url, payload, auth=False)
        return body["message"]["content"]

    def _openai_chat(self, messages: list[dict], system: str) -> str:
        url = f"{self.base_url}/chat/completions"
        full_messages = [{"role": "system", "content": system}] + messages
        payload = {"model": self.model, "messages": full_messages}
        body = self._post(url, payload, auth=bool(self.api_key))
        return body["choices"][0]["message"]["content"]

    def _post(self, url: str, payload: dict, auth: bool = False) -> dict:
        data = json.dumps(payload).encode("utf-8")
        headers = {"Content-Type": "application/json"}
        if auth and self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        from backend.config import llm_request_timeout
        req = urllib.request.Request(url, data=data, headers=headers, method="POST")
        try:
            with urllib.request.urlopen(req, timeout=llm_request_timeout()) as resp:
                return json.loads(resp.read())
        except urllib.error.URLError as e:
            raise ConnectionError(
                f"Cannot reach LLM at {url}.\n"
                f"Is your local LLM running? (e.g. `ollama serve`)\n"
                f"Error: {e}"
            )


def parse_response(response: str) -> tuple[str, list[str]]:
    """Parse simple ---EXPLANATION--- / ---COMMANDS--- format."""
    explanation, commands = "", []
    try:
        exp_start = response.index("---EXPLANATION---") + len("---EXPLANATION---")
        exp_end   = response.index("---COMMANDS---")
        cmd_start = exp_end + len("---COMMANDS---")
        cmd_end   = response.index("---END---")

        explanation = response[exp_start:exp_end].strip()
        for line in response[cmd_start:cmd_end].strip().splitlines():
            line = line.strip().lstrip("$").strip()
            if line and line.upper() != "NONE":
                commands.append(line)
    except ValueError:
        explanation = response

    return explanation, commands
