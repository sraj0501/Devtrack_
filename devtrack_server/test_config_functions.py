#!/usr/bin/env python3
"""Quick test of new config functions."""

from backend.config import (
    http_timeout,
    http_timeout_short,
    http_timeout_long,
    ipc_retry_delay_ms,
    llm_request_timeout,
    sentiment_analysis_window_minutes,
    lmstudio_host,
    git_sage_default_model,
    prompt_timeout_simple,
    prompt_timeout_work,
    prompt_timeout_task,
    ipc_connect_timeout_secs,
)

# Test all new config functions
print("Testing new config functions...")

# Timeouts
assert http_timeout_short() == 10, f"Expected 10, got {http_timeout_short()}"
assert http_timeout() == 30, f"Expected 30, got {http_timeout()}"
assert http_timeout_long() == 60, f"Expected 60, got {http_timeout_long()}"

# Delays
assert ipc_retry_delay_ms() == 2000, f"Expected 2000, got {ipc_retry_delay_ms()}"

# LLM timeouts
assert llm_request_timeout() == 120, f"Expected 120, got {llm_request_timeout()}"

# Sentiment window
assert sentiment_analysis_window_minutes() == 120, f"Expected 120, got {sentiment_analysis_window_minutes()}"

# Hosts
assert "localhost" in lmstudio_host().lower() or "127" in lmstudio_host(), f"Unexpected LM Studio host: {lmstudio_host()}"

# Models
assert "llama" in git_sage_default_model().lower(), f"Unexpected Git Sage model: {git_sage_default_model()}"

# Prompt timeouts
assert prompt_timeout_simple() == 30, f"Expected 30, got {prompt_timeout_simple()}"
assert prompt_timeout_work() == 60, f"Expected 60, got {prompt_timeout_work()}"
assert prompt_timeout_task() == 120, f"Expected 120, got {prompt_timeout_task()}"

print("✓ All config functions work correctly!")
print("\nValues loaded from .env:")
print(f"  http_timeout_short: {http_timeout_short()} seconds")
print(f"  http_timeout: {http_timeout()} seconds")
print(f"  http_timeout_long: {http_timeout_long()} seconds")
print(f"  ipc_retry_delay_ms: {ipc_retry_delay_ms()} ms")
print(f"  llm_request_timeout: {llm_request_timeout()} seconds")
print(f"  sentiment_analysis_window_minutes: {sentiment_analysis_window_minutes()} minutes")
print(f"  lmstudio_host: {lmstudio_host()}")
print(f"  git_sage_default_model: {git_sage_default_model()}")
print(f"  prompt_timeout_simple: {prompt_timeout_simple()} seconds")
print(f"  prompt_timeout_work: {prompt_timeout_work()} seconds")
print(f"  prompt_timeout_task: {prompt_timeout_task()} seconds")
