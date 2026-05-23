"""LLM backend supporting Ollama and OpenAI-compatible APIs (OpenAI, Groq, LMStudio).

Ollama uses urllib (no SDK needed — local only).
All OpenAI-compatible providers (openai, groq, lmstudio, custom) use the openai SDK,
which handles auth, retries, and works through Cloudflare-protected endpoints.
"""
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

        from backend.config import ollama_host, lmstudio_host, groq_host
        defaults = {
            "ollama":   ollama_host(),
            "openai":   "https://api.openai.com/v1",
            "lmstudio": lmstudio_host(),
            "groq":     groq_host(),
        }
        self.base_url = base_url or defaults.get(provider, ollama_host())

    # ── Public methods ────────────────────────────────────────────────────────

    def ask(self, user_message: str, context: str) -> str:
        """Simple one-shot ask (used by non-agent mode)."""
        full = f"{context}\n\nUser request: {user_message}"
        messages = [{"role": "user", "content": full}]
        return self.raw_chat(messages, SIMPLE_SYSTEM_PROMPT)

    def raw_chat(self, messages: list[dict], system_prompt: str,
                 json_mode: bool = False) -> str:
        """Send a full conversation history and return the assistant reply.

        json_mode=True enforces structured JSON output (used by the agent loop).
        """
        if self.provider == "ollama":
            return self._ollama_chat(messages, system_prompt, json_mode)
        else:
            return self._openai_sdk_chat(messages, system_prompt, json_mode)

    # ── Provider implementations ──────────────────────────────────────────────

    def _ollama_chat(self, messages: list[dict], system: str,
                     json_mode: bool = False) -> str:
        """Ollama via urllib — no SDK required for local calls."""
        url = f"{self.base_url}/api/chat"
        full_messages = [{"role": "system", "content": system}] + messages
        payload = {"model": self.model, "messages": full_messages, "stream": False}
        if json_mode:
            payload["format"] = "json"
        body = self._urllib_post(url, payload, auth=False)
        return body["message"]["content"]

    def _openai_sdk_chat(self, messages: list[dict], system: str,
                         json_mode: bool = False) -> str:
        """OpenAI-compatible chat via the openai SDK (works for openai, groq, lmstudio)."""
        try:
            import openai
        except ImportError:
            raise ConnectionError(
                "The 'openai' package is required for non-Ollama providers.\n"
                "Install it with: uv add openai"
            )
        from backend.config import llm_request_timeout
        full_messages = [{"role": "system", "content": system}] + messages
        # Strip LiteLLM-style provider prefix (e.g. "groq/compound-beta" → "compound-beta")
        model = self.model.split("/", 1)[-1] if "/" in self.model else self.model
        try:
            client = openai.OpenAI(
                api_key=self.api_key or "no-key",
                base_url=self.base_url,
                timeout=llm_request_timeout(),
            )
            kwargs: dict = {"model": model, "messages": full_messages}
            if json_mode:
                kwargs["response_format"] = {"type": "json_object"}
            try:
                resp = client.chat.completions.create(**kwargs)
            except openai.BadRequestError:
                # Model doesn't support response_format — retry without it
                kwargs.pop("response_format", None)
                resp = client.chat.completions.create(**kwargs)
            return resp.choices[0].message.content
        except openai.AuthenticationError as e:
            raise ConnectionError(f"Authentication failed for {self.provider}: {e}")
        except openai.APIConnectionError as e:
            raise ConnectionError(
                f"Cannot reach LLM at {self.base_url}.\n"
                f"Is your provider running/reachable?\n"
                f"Error: {e}"
            )
        except Exception as e:
            raise ConnectionError(f"LLM request failed ({self.provider}): {e}")

    def _urllib_post(self, url: str, payload: dict, auth: bool = False) -> dict:
        """Raw urllib POST — used only for Ollama (local, no Cloudflare)."""
        data = json.dumps(payload).encode("utf-8")
        headers = {"Content-Type": "application/json", "User-Agent": "devtrack/1.0"}
        if auth and self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"

        from backend.config import llm_request_timeout
        req = urllib.request.Request(url, data=data, headers=headers, method="POST")
        try:
            with urllib.request.urlopen(req, timeout=llm_request_timeout()) as resp:
                return json.loads(resp.read())
        except urllib.error.URLError as e:
            raise ConnectionError(
                f"Cannot reach Ollama at {url}.\n"
                f"Is Ollama running? Try: ollama serve\n"
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
