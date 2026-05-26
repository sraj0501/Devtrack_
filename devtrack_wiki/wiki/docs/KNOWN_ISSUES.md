# Known Issues & Debugging Notes

## AI Enhancement Intermittent Failure

**Status:** Investigated, root cause identified

**Issue:** The `devtrack git commit` wrapper's AI enhancement sometimes fails silently with "AI enhancement failed" message, even though the LLM provider is available.

### Investigation Results

- ✅ Ollama is running and responsive (`curl http://localhost:11434/api/tags` works)
- ✅ Ollama API responds correctly to test prompts (curl returns proper JSON responses)
- ✅ `backend.ai.ollama_client.generate()` works when called directly
- ✅ `backend.llm.OllamaProvider` works when tested in isolation
- ✅ Provider chain works when tested directly
- ⚠️ `CommitMessageEnhancer.enhance_message_with_ai()` returns original message unchanged
- ⚠️ The wrapper script's grep check for "enhanced" in stdout doesn't find it (because Python logging goes to stderr, not stdout)

### Root Causes

1. **Stdout/Stderr Issue (FIXED)**
   - The `commit_message_enhancer.py` script only logs via Python's logging module (stderr)
   - The wrapper script checks stdout for the word "enhanced": `if echo "$ENHANCEMENT_OUTPUT" | grep -q "enhanced"`
   - **Fix applied:** Added `print("enhanced", file=sys.stdout)` when enhancement succeeds
   - **Status:** Fix is in place but needs testing with actual git commits

2. **Enhancement Logic Issue (NOT FULLY RESOLVED)**
   - The `enhance_message_with_ai()` method is returning the original message unchanged
   - Direct LLM calls work, but something in the enhancement logic is swallowing the response
   - Possible causes:
     - The LLM response is being filtered/rejected by the cleanup logic
     - An exception is being caught silently
     - The prompt is malformed

### Recommendations

1. **Short term:** Test with actual commits to verify the stdout fix works
2. **Medium term:** Add detailed logging to `enhance_message_with_ai()` to track:
   - Whether LLM is called
   - What prompt is sent
   - What response is received
   - What filtering/rejection conditions are met
3. **Long term:** Consider moving enhancement to be optional (with fallback to original message)

### Files Modified

- `backend/commit_message_enhancer.py`: Added stdout output on success
- `devtrack-git-wrapper.sh`: Already checks for "enhanced" in output (uses `grep -q`)

### Testing Checklist

- [ ] Test `devtrack git commit -m "message"` with actual staged changes
- [ ] Verify "✓ AI-enhanced commit message:" appears
- [ ] Check that message is actually enhanced (not original)
- [ ] Test with different types of commits (bugfix, feature, docs, etc.)
- [ ] Verify fallback to original message works when enhancement fails
