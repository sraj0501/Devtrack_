# LLM Configuration & Provider Guide

Complete guide to configuring and using different AI providers with DevTrack.

---

## Overview

DevTrack uses a flexible LLM provider system with automatic fallback:

1. **Primary Provider**: Your chosen LLM (Ollama, OpenAI, or Anthropic)
2. **Fallback Chain**: Automatically try other available providers if primary is down
3. **Graceful Degradation**: Features degrade instead of crashing if no AI available

This means you can:
- Start with local Ollama (free, 100% offline)
- Add OpenAI or Anthropic for better quality
- Seamlessly switch between them
- Optimize for cost vs. quality

---

## Provider Comparison

| Aspect | Ollama | OpenAI | Anthropic | LMStudio |
|--------|--------|--------|-----------|----------|
| **Cost** | Free | $0.03-0.06/1K tokens | $0.003-0.06/1K tokens | Free |
| **Speed** | Depends on hardware | Fast (network) | Fast (network) | Depends on hardware |
| **Quality** | Good | Excellent (GPT-4) | Excellent (Claude) | Good-Excellent |
| **Offline** | Yes | No | No | Yes |
| **Privacy** | 100% local | Sent to OpenAI | Sent to Anthropic | 100% local |
| **Setup** | Easy | Need API key | Need API key | Easy |
| **Models** | llama2, mistral, etc. | GPT-4, GPT-3.5 | Claude 3 | Local HF models |

---

## Option 1: Ollama (Recommended for Local/Privacy)

Local LLM that runs on your machine. Free and 100% offline.

### Installation

```bash
# macOS
brew install ollama
brew services start ollama

# Linux
curl -fsSL https://ollama.com/install.sh | sh
ollama serve

# Windows
# Download from https://ollama.com/download
```

### Configuration

In `.env`:

```bash
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral
# Or: llama2, neural-chat, orca-mini, codellama
```

### Model Selection

Common models for DevTrack:

| Model | Size | Speed | Quality | Use Case |
|-------|------|-------|---------|----------|
| **mistral** | 7B | Fast | Good | Default, best balance |
| **llama2** | 7B-13B | Medium | Good | Reliable, well-tested |
| **neural-chat** | 7B | Fast | Good | Conversation-optimized |
| **orca-mini** | 3B | Very fast | Basic | For slow hardware |
| **codellama** | 7B-34B | Slow | Excellent | Git/code focused |

### Download and Run

```bash
# Download model
ollama pull mistral

# Test it works
ollama run mistral "What is DevTrack?"

# Keep running in background
# It starts automatically on system boot if installed via brew
ollama serve
```

### Tips for Performance

```bash
# Use smaller model for faster responses
OLLAMA_MODEL=orca-mini

# Or use quantized version
OLLAMA_MODEL=mistral:q4_K_M  # Quantized 4-bit

# Increase context window for better responses
curl http://localhost:11434/api/generate -d '{
  "model": "mistral",
  "prompt": "test",
  "context": 4096
}'
```

### Troubleshooting Ollama

```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# View available models
ollama list

# Pull a model
ollama pull mistral

# Check logs
tail -f /var/log/ollama.log

# Restart
brew services restart ollama  # macOS
sudo systemctl restart ollama  # Linux
```

---

## Option 2: OpenAI (Best Quality)

Cloud-based GPT-4 and GPT-3.5. Superior quality but costs money.

### Setup

1. **Create an OpenAI account**: https://openai.com/api/
2. **Generate API key**: https://platform.openai.com/api-keys
3. **Add to .env**:

```bash
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-...your-key-here...
OPENAI_MODEL=gpt-4
# Or: gpt-4-turbo-preview, gpt-3.5-turbo
```

### Model Selection

| Model | Cost/1K tokens | Speed | Quality | Best For |
|-------|---|---|---|---|
| **gpt-4** | $0.03 (input) / $0.06 (output) | Slow | Excellent | Complex tasks, reasoning |
| **gpt-4-turbo** | $0.01 / $0.03 | Fast | Excellent | Faster responses, less cost |
| **gpt-3.5-turbo** | $0.0005 / $0.0015 | Very fast | Good | Budget-conscious, simple tasks |

### Cost Estimation

For DevTrack use cases:

- **Commit enhancement**: ~200 tokens per commit (daily avg: 3 commits = 600 tokens = $0.02/day)
- **Work update enhancement**: ~150 tokens per update (daily avg: 5 updates = 750 tokens = $0.02/day)
- **Report generation**: ~500 tokens per report (weekly avg: 2 reports = 1000 tokens = $0.04/week)

**Monthly estimate**: ~$2-5 depending on usage

### Configuration

```bash
# Use GPT-4 for best quality
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-...
OPENAI_MODEL=gpt-4
OPENAI_TEMPERATURE=0.7  # 0=deterministic, 1=creative

# Or use GPT-3.5 for cheaper option
OPENAI_MODEL=gpt-3.5-turbo
```

### Monitoring Costs

```bash
# Check your usage on OpenAI dashboard
# https://platform.openai.com/account/billing/overview

# Estimate current month
# Go to: https://platform.openai.com/account/billing/limits
```

---

## Option 3: Anthropic (Claude API)

Cloud-based Claude API. Great reasoning and longer context.

### Setup

1. **Create Anthropic account**: https://console.anthropic.com/
2. **Generate API key**: https://console.anthropic.com/account/keys
3. **Add to .env**:

```bash
LLM_PROVIDER=anthropic
ANTHROPIC_API_KEY=sk-ant-...your-key-here...
ANTHROPIC_MODEL=claude-3-opus
# Or: claude-3-sonnet, claude-3-haiku
```

### Model Selection

| Model | Cost/1M tokens | Quality | Speed | Best For |
|-------|---|---|---|---|
| **claude-3-opus** | Input: $15, Output: $75 | Excellent | Slow | Complex reasoning |
| **claude-3-sonnet** | Input: $3, Output: $15 | Good | Fast | Balanced |
| **claude-3-haiku** | Input: $0.25, Output: $1.25 | Good | Very fast | Budget |

### Configuration

```bash
# Use Claude 3 for excellent reasoning
LLM_PROVIDER=anthropic
ANTHROPIC_API_KEY=sk-ant-...
ANTHROPIC_MODEL=claude-3-opus
ANTHROPIC_MAX_TOKENS=1024
```

---

## Option 4: Hybrid Setup (Recommended for Production)

Use multiple providers with smart fallback:

### Configuration

```bash
# Primary (prefer local for privacy)
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral

# Fallback 1 (use if Ollama unavailable)
OPENAI_API_KEY=sk-...

# Fallback 2 (use if both above fail)
ANTHROPIC_API_KEY=sk-ant-...
```

### How It Works

```python
# backend/llm/provider_factory.py
def get_llm_provider():
    # Try primary
    if LLM_PROVIDER == "ollama" and ollama.is_available():
        return OllamaProvider()

    # Try fallback 1
    if OPENAI_API_KEY and openai.is_available():
        return OpenAIProvider()

    # Try fallback 2
    if ANTHROPIC_API_KEY and anthropic.is_available():
        return AnthropicProvider()

    # Last resort
    return OllamaProvider()  # Local fallback
```

### Advantages

- **Privacy**: Uses local Ollama by default
- **Reliability**: Falls back to APIs if Ollama down
- **Cost**: Only uses APIs when needed
- **Performance**: Uses best available provider
- **Resilience**: Always has a working option

---

## Advanced: Custom Configuration

### Per-Feature Provider Selection

```bash
# Use different providers for different features
COMMIT_ENHANCEMENT_PROVIDER=openai      # For best quality
WORK_UPDATE_PROVIDER=ollama             # For privacy
REPORT_GENERATION_PROVIDER=anthropic    # For reasoning

# Or use same for all
DEFAULT_LLM_PROVIDER=ollama
```

### Temperature and Response Tuning

```bash
# Temperature: 0=deterministic, 1=creative
OPENAI_TEMPERATURE=0.3              # For factual tasks
OLLAMA_TEMPERATURE=0.7              # For creative tasks

# Top P: Nucleus sampling
OPENAI_TOP_P=0.9

# Max tokens
OPENAI_MAX_TOKENS=1024
ANTHROPIC_MAX_TOKENS=2048
```

### Using OpenAI-Compatible Endpoints

```bash
# Use LMStudio or other OpenAI-compatible servers
LLM_PROVIDER=openai
OPENAI_BASE_URL=http://localhost:8000  # LMStudio
OPENAI_API_KEY=lm-studio               # Dummy key
OPENAI_MODEL=local-model
```

---

## Cost Optimization

### Strategy 1: Local-First (Free)

```bash
LLM_PROVIDER=ollama
OLLAMA_MODEL=orca-mini  # Smaller, faster
```

**Cost**: $0/month
**Quality**: Basic, good for simple tasks
**Speed**: Very fast
**Privacy**: 100% local

### Strategy 2: Hybrid with Smart Fallback ($2-5/month)

```bash
LLM_PROVIDER=ollama              # Primary (free)
OPENAI_API_KEY=sk-...             # Fallback (pay when needed)
OPENAI_MODEL=gpt-3.5-turbo        # Cheap model
```

**Cost**: ~$3/month (only when Ollama unavailable)
**Quality**: Good (local) + Good (OpenAI when needed)
**Speed**: Fast
**Privacy**: Local-first

### Strategy 3: Premium Quality ($10-20/month)

```bash
LLM_PROVIDER=openai
OPENAI_MODEL=gpt-4
```

**Cost**: ~$15/month (high usage)
**Quality**: Excellent
**Speed**: Medium
**Privacy**: Cloud-based

### Strategy 4: Budget Conscious ($1-2/month)

```bash
LLM_PROVIDER=openai
OPENAI_MODEL=gpt-3.5-turbo
```

**Cost**: ~$1/month (light usage)
**Quality**: Good
**Speed**: Very fast
**Privacy**: Cloud-based

---

## Monitoring & Debugging

### Check Current Provider

```bash
# View which provider is active
uv run python -c "from backend.llm.provider_factory import get_llm_provider; print(get_llm_provider())"

# Check provider status
curl http://localhost:11434/api/tags              # Ollama
curl -H "Authorization: Bearer $OPENAI_API_KEY" https://api.openai.com/v1/models | jq .  # OpenAI
```

### Test Each Provider

```bash
# Test Ollama
uv run python -c "
from backend.llm.ollama_provider import OllamaProvider
p = OllamaProvider()
print(p.generate('Hello, how are you?'))
"

# Test OpenAI
uv run python -c "
from backend.llm.openai_provider import OpenAIProvider
p = OpenAIProvider()
print(p.generate('Hello, how are you?'))
"

# Test Anthropic
uv run python -c "
from backend.llm.anthropic_provider import AnthropicProvider
p = AnthropicProvider()
print(p.generate('Hello, how are you?'))
"
```

### View Logs

```bash
# Watch AI interactions
tail -f Data/logs/python_bridge.log | grep -i "llm\|enhancement"

# Debug provider selection
tail -f Data/logs/python_bridge.log | grep -i "provider"
```

### Reset Provider Cache

```bash
# If provider selection is stuck
uv run python -c "from backend.llm.provider_factory import reset_provider_cache; reset_provider_cache()"

# Then restart daemon
devtrack restart
```

---

## Troubleshooting

### Ollama Not Responding

```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# Restart Ollama
brew services restart ollama  # macOS
sudo systemctl restart ollama  # Linux

# Check Ollama logs
tail -f /var/log/ollama.log
```

### OpenAI API Errors

```bash
# Check API key is correct
echo $OPENAI_API_KEY

# Verify API key format (should start with 'sk-')
# Go to: https://platform.openai.com/api-keys

# Check rate limits
# Go to: https://platform.openai.com/account/rate-limits

# Test API directly
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

### Anthropic API Errors

```bash
# Check API key
echo $ANTHROPIC_API_KEY

# Verify format (should start with 'sk-ant-')
# Generate new key at: https://console.anthropic.com/account/keys
```

### All Providers Down

```bash
# DevTrack degrades gracefully
# AI-powered features (enhancement, reports) will be disabled
# Core features (monitoring, parsing, integrations) still work

# Check which providers are available
uv run python -c "
from backend.llm.provider_factory import get_available_providers
print(get_available_providers())
"

# Try recovering
devtrack restart
```

---

## Migration: Switching Providers

### From Ollama to OpenAI

```bash
# Edit .env
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-...
OPENAI_MODEL=gpt-4

# Restart
devtrack restart

# Verify
curl https://api.openai.com/v1/models -H "Authorization: Bearer $OPENAI_API_KEY"
```

### From OpenAI to Anthropic

```bash
# Edit .env
LLM_PROVIDER=anthropic
ANTHROPIC_API_KEY=sk-ant-...
ANTHROPIC_MODEL=claude-3-opus

# Remove OpenAI key (optional)
# OPENAI_API_KEY=

# Restart
devtrack restart
```

### From Cloud Back to Local

```bash
# Edit .env
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral

# Start Ollama if not running
ollama serve

# Restart DevTrack
devtrack restart
```

---

## Performance Tuning

### For Speed

```bash
# Use smaller model
OLLAMA_MODEL=orca-mini

# Use fast cloud model
OPENAI_MODEL=gpt-3.5-turbo

# Reduce max tokens
OPENAI_MAX_TOKENS=256
```

### For Quality

```bash
# Use larger model
OLLAMA_MODEL=codellama

# Use best cloud model
OPENAI_MODEL=gpt-4

# Increase max tokens
OPENAI_MAX_TOKENS=2048
```

### For Cost

```bash
# Use local Ollama
LLM_PROVIDER=ollama

# Or use cheapest cloud model
OPENAI_MODEL=gpt-3.5-turbo

# Limit usage
COMMIT_ENHANCEMENT_ENABLED=true
REPORT_GENERATION_ENABLED=true
WORK_UPDATE_ENHANCEMENT_ENABLED=false  # Can be expensive
```

---

## Next Steps

- **Installation**: See [Installation Guide](INSTALLATION.md)
- **Git Features**: See [Git Features Guide](GIT_FEATURES.md)
- **Architecture**: See [Architecture Overview](ARCHITECTURE.md)
- **Troubleshooting**: See [Troubleshooting Guide](TROUBLESHOOTING.md)

---

## FAQ

**Q: Can I use multiple providers simultaneously?**
A: Yes, use the fallback chain configuration. Primary is tried first, then fallbacks.

**Q: Does DevTrack send my commit messages to the cloud?**
A: Only if you configure a cloud provider (OpenAI/Anthropic). With Ollama, everything is local.

**Q: Can I switch providers without restarting?**
A: You need to restart (`devtrack restart`) for changes to take effect.

**Q: What if I run out of API quota?**
A: DevTrack will automatically fall back to the next provider in the chain. If only cloud providers available, AI features will be disabled.

**Q: Can I use my own LLM server?**
A: Yes, use the OpenAI-compatible endpoint feature to point to LMStudio or other compatible servers.

**Q: How much does it cost to run DevTrack with OpenAI?**
A: ~$2-5 per month for typical developer usage (3-5 commits/day, 5 work updates/day).
