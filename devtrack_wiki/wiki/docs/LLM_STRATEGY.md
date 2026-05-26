# Hybrid LLM Strategy: Offline + Commercial APIs

**Vision**: DevTrack with optional commercial AI for enhanced capabilities
**Status**: Foundation in place, ready for full implementation
**Current**: Ollama (local) with OpenAI/Anthropic fallback
**Future**: User-controlled hybrid switching, cost optimization, quality tiers

---

## 🎯 Core Principle

**"Start local, upgrade optionally"**

- Default: Ollama (100% offline, free)
- Optional: OpenAI (GPT-4, superior quality, costs money)
- Optional: Claude API (Anthropic, better reasoning, costs money)
- Optional: Custom (LMStudio, HuggingFace, etc.)
- Always: Graceful fallback to local if API unavailable

---

## 🏗️ Architecture

### Current Implementation (Phase 1-3)

```python
# backend/llm/provider_factory.py
def get_llm_provider():
    """
    Returns LLM provider with fallback chain:
    1. Primary (user choice): OpenAI, Anthropic, or Ollama
    2. Fallback 1: If primary unavailable, try next in chain
    3. Fallback 2: Eventually fallback to Ollama (always works offline)
    """

    primary = get_primary_provider()  # User configured
    fallbacks = get_fallback_chain()  # Auto-configured

    for provider in [primary] + fallbacks:
        if provider.is_available():
            return provider

    return OllamaProvider()  # Last resort
```

### Enhanced Implementation (Phase 4+)

```
┌─────────────────────────────────────────────┐
│ DevTrack Core Features                      │
│ (Projects, Tasks, Sprints, etc.)            │
└────────────────┬────────────────────────────┘
                 │ Requests AI
                 ▼
┌─────────────────────────────────────────────┐
│ LLM Selection Layer                         │
│ ├─ User preference (config)                 │
│ ├─ Feature requirements (what quality)      │
│ ├─ Cost budget (if using APIs)              │
│ └─ Availability (fallback if needed)        │
└────────────┬────────────────────────────────┘
             │
    ┌────────┴────────┬──────────┬──────────┐
    ▼                 ▼          ▼          ▼
┌──────────┐    ┌───────────┐ ┌──────┐ ┌──────────┐
│ Ollama   │    │ OpenAI    │ │Claude│ │LMStudio  │
│(Offline) │    │ (GPT-4)   │ │(API) │ │(Local)   │
│  Free    │    │  Paid     │ │Paid  │ │ Free     │
└──────────┘    └───────────┘ └──────┘ └──────────┘
    │                │          │          │
    └────────────────┴──────────┴──────────┘
                     │
                     ▼
            Response Processing
```

---

## 🔄 Provider Options

### 1. Ollama (Local, Default)
```
Model: llama2, mistral, neural-chat, etc.
Cost: $0 (runs locally)
Speed: Fast (depends on hardware)
Quality: Good for basic tasks
Availability: 100% offline
Best for: Default, fallback, privacy-critical tasks

Configuration:
OLLAMA_URL=http://localhost:11434
OLLAMA_MODEL=mistral
```

### 2. OpenAI (API)
```
Models: GPT-4, GPT-4-Turbo, GPT-3.5-Turbo
Cost: $0.03-0.06 per 1K tokens
Speed: Fast (network dependent)
Quality: Excellent
Availability: Requires API key + internet
Best for: Complex reasoning, accuracy-critical

Configuration:
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-...
OPENAI_MODEL=gpt-4-turbo-preview
```

### 3. Claude API (Anthropic)
```
Models: Claude 3 (Opus, Sonnet, Haiku)
Cost: $0.003-0.024 per 1K tokens
Speed: Fast (network dependent)
Quality: Excellent (superior reasoning)
Availability: Requires API key + internet
Best for: Complex tasks, long context

Configuration:
LLM_PROVIDER=anthropic
ANTHROPIC_API_KEY=sk-ant-...
ANTHROPIC_MODEL=claude-3-opus-20240229
```

### 4. LMStudio (Local, Alternative)
```
Models: Any HuggingFace GGUF model
Cost: $0 (runs locally)
Speed: Depends on hardware
Quality: Variable (model dependent)
Availability: 100% offline
Best for: Privacy, custom models

Configuration:
LLM_PROVIDER=lmstudio
LMSTUDIO_URL=http://localhost:1234
LMSTUDIO_MODEL=model-name
```

### 5. Custom/Self-Hosted
```
Options: Llama 2, Mistral, Custom servers
Cost: Server costs or free
Speed: Depends on setup
Quality: Variable
Availability: Depends on setup
Best for: Organizations with privacy requirements

Configuration:
LLM_PROVIDER=custom
CUSTOM_API_URL=https://your-server.com/api
CUSTOM_API_KEY=...
```

---

## 🎛️ User Configuration

### Config File Approach

```yaml
# ~/.devtrack/llm.yaml
llm:
  # Primary provider (what to use first)
  primary:
    provider: openai
    model: gpt-4-turbo-preview
    api_key: sk-...
    max_cost_per_day: 10.00  # Safety limit

  # Fallback chain (use if primary unavailable)
  fallbacks:
    - provider: anthropic
      model: claude-3-sonnet
      api_key: sk-ant-...

    - provider: ollama
      model: mistral
      url: http://localhost:11434

  # Feature-specific overrides
  features:
    # Use cheaper model for routine tasks
    work_update_parsing:
      provider: openai
      model: gpt-3.5-turbo

    # Use most capable for complex analysis
    project_planning:
      provider: openai
      model: gpt-4-turbo-preview

    # Always local for privacy
    knowledge_base:
      provider: ollama
      model: mistral

  # Cost management
  cost_limits:
    daily: 10.00
    monthly: 200.00
    per_feature:
      project_planning: 5.00
      analytics: 3.00
      task_assistant: 2.00
```

### CLI Configuration

```bash
# Set primary provider
devtrack config set llm.primary.provider openai
devtrack config set llm.primary.model gpt-4-turbo-preview
devtrack config set llm.primary.api_key sk-...

# Add fallback
devtrack config add llm.fallbacks anthropic
devtrack config set llm.fallbacks.anthropic.api_key sk-ant-...

# Set feature-specific provider
devtrack config set llm.features.project_planning.provider openai

# Set cost limits
devtrack config set llm.cost_limits.daily 10.00
devtrack config set llm.cost_limits.monthly 200.00

# Show current config
devtrack config show llm
devtrack config show llm.cost_limits

# Test provider
devtrack llm test-provider openai
devtrack llm test-provider anthropic
devtrack llm test-provider ollama
```

### Interactive Setup

```bash
$ devtrack llm setup

Welcome to DevTrack LLM Setup!

1. Offline Only (Ollama)
   - Free
   - Private
   - Works offline
   - Good quality

2. Hybrid (Local + API Fallback)
   - Free default
   - Better quality with API
   - Graceful fallback
   - Recommended

3. API-First (Cloud LLMs)
   - Best quality
   - Costs money
   - Requires internet
   - Premium experience

Which setup would you like? [1/2/3]: 2

Great! Let's set up hybrid mode.

Which API would you like to use as fallback?
1. OpenAI (GPT-4)
2. Claude (Anthropic)
3. Both (try Claude first)
4. Skip for now

Select: 3

Enter your Claude API key: sk-ant-...
Enter your OpenAI API key: sk-...

Setting up cost limits...
Daily budget: $10 (default)
Confirm? [y/n]: y

Testing configuration...
✓ Ollama responding
✓ Claude API working
✓ OpenAI API working

All set! Your config:
- Primary: Claude (Opus)
- Fallback: OpenAI (GPT-4)
- Last resort: Ollama

Start using: devtrack start
```

---

## 🧠 Provider Selection Strategy

### By Feature Complexity

```
Simple Tasks (Use Cheaper)
├─ Work update parsing → OpenAI GPT-3.5 ($0.001)
├─ Commit message enhancement → Ollama (free)
└─ Basic categorization → Ollama (free)

Medium Tasks (Use Good)
├─ Story estimation → OpenAI GPT-4 ($0.03)
├─ Acceptance criteria generation → Claude Sonnet ($0.003)
└─ Risk assessment → OpenAI GPT-4 ($0.03)

Complex Tasks (Use Best)
├─ Project planning → Claude Opus ($0.015)
├─ Scenario analysis → Claude Opus ($0.015)
├─ Architectural decisions → Claude Opus ($0.015)
└─ Knowledge synthesis → OpenAI GPT-4 ($0.03)

Privacy-Critical (Always Local)
├─ Sensitive project details → Ollama
├─ Internal knowledge base → Ollama
├─ Proprietary code analysis → Ollama
└─ Team member data → Ollama
```

### Automatic Provider Selection

```python
# backend/llm/provider_selector.py
class ProviderSelector:
    def select_provider(self, task_type: str, complexity: str) -> LLMProvider:
        """
        Automatically select best provider based on:
        1. User configuration (primary preference)
        2. Task requirements (accuracy vs speed)
        3. Complexity level
        4. Cost constraints
        5. Privacy requirements
        6. Availability
        """

        if task_type == "work_update_parsing":
            # Low complexity, frequency high, cost critical
            return self.get_cheapest_available()

        elif task_type == "project_planning":
            # High complexity, accuracy critical
            return self.get_best_available()

        elif task_type.contains("sensitive"):
            # Privacy critical
            return OllamaProvider()  # Always local

        else:
            # Default: user preference with fallback
            return self.get_user_preferred()

    def get_cheapest_available(self) -> LLMProvider:
        """Return cheapest provider that's available"""
        providers = [
            (OllamaProvider(), 0),          # Free
            (OpenAIProvider("gpt-3.5"), 1), # Cheap
            (ClaudeProvider("haiku"), 2),   # Cheap
        ]

        for provider, cost in providers:
            if provider.is_available():
                return provider

        return OllamaProvider()  # Fallback

    def get_best_available(self) -> LLMProvider:
        """Return best quality provider that's available"""
        providers = [
            (ClaudeProvider("opus"), 1),        # Best reasoning
            (OpenAIProvider("gpt-4"), 2),       # Very good
            (OpenAIProvider("gpt-3.5"), 3),     # Good
            (OllamaProvider(), 4),              # Fallback
        ]

        for provider, quality in providers:
            if provider.is_available():
                return provider

        return OllamaProvider()  # Fallback
```

---

## 💰 Cost Management

### Built-in Cost Controls

```python
# backend/llm/cost_tracker.py
class CostTracker:
    def can_use_provider(self, provider: str, tokens: int) -> bool:
        """Check if using this provider stays within budget"""

        estimated_cost = provider.estimate_cost(tokens)
        daily_spent = self.get_daily_spending()
        monthly_spent = self.get_monthly_spending()
        feature_spent = self.get_feature_spending()

        daily_limit = config.get("llm.cost_limits.daily")
        monthly_limit = config.get("llm.cost_limits.monthly")
        feature_limit = config.get(f"llm.cost_limits.{feature}")

        if daily_spent + estimated_cost > daily_limit:
            logger.warning(f"Daily budget exceeded")
            return False

        if monthly_spent + estimated_cost > monthly_limit:
            logger.warning(f"Monthly budget exceeded")
            return False

        if feature_spent + estimated_cost > feature_limit:
            logger.warning(f"Feature budget exceeded")
            return False

        return True

    def warn_on_high_cost(self, provider: str, tokens: int):
        """Warn user before expensive operation"""
        cost = provider.estimate_cost(tokens)

        if cost > 0.10:  # More than 10 cents
            logger.warning(f"⚠️  This operation may cost ${cost:.2f}")
            return self.confirm_with_user()

        return True
```

### Dashboard & Monitoring

```bash
$ devtrack llm stats

LLM Usage & Costs
═════════════════

Today (Mar 11, 2026)
├─ OpenAI GPT-4: $2.45 (1240 tokens, 5 requests)
├─ Claude Opus: $0.87 (340 tokens, 2 requests)
├─ Ollama: $0.00 (1800 tokens, 8 requests)
└─ Daily total: $3.32 / $10.00 budget (33%)

This Week
├─ Daily average: $2.10
├─ Peak day: $4.50
└─ Week total: $14.70 / $200.00 budget (7%)

By Feature
├─ Project planning: $8.50 (68%)
├─ Task assistant: $3.20 (26%)
├─ Work updates: $0.80 (6%)
└─ Other: $0.20 (0%)

Provider Usage
├─ OpenAI (47%): GPT-4, GPT-3.5
├─ Claude (31%): Opus, Sonnet
└─ Ollama (22%): Free fallback

Recommendations
├─ 💡 Use GPT-3.5 for work updates (save 90%)
├─ 💡 Consider Claude Sonnet instead of Opus (save 40%)
└─ ✅ Good balance between cost and quality
```

---

## 🔄 Fallback & Error Handling

### Automatic Fallback Strategy

```python
# backend/llm/fallback_handler.py
class FallbackHandler:
    async def call_llm_with_fallback(self, task, config):
        """
        Try providers in order:
        1. Primary (user choice)
        2. Fallback chain
        3. Degraded mode (if available)
        4. Last resort (Ollama)
        """

        providers = self.build_provider_chain(config)
        last_error = None

        for provider in providers:
            try:
                result = await provider.call(task)
                return result

            except APIError as e:
                last_error = e
                logger.debug(f"{provider.name} failed: {e}")

                # Notify user of fallback
                if provider == providers[0]:  # Primary failed
                    logger.warning(f"Primary provider failed, using {providers[1].name}")

                continue

            except RateLimitError as e:
                last_error = e
                logger.warning(f"Rate limited on {provider.name}, fallback")
                continue

            except BudgetExceededError as e:
                last_error = e
                logger.warning(f"Budget exceeded on {provider.name}, fallback")
                continue

        # All providers failed
        if last_error:
            logger.error(f"All LLM providers failed: {last_error}")
            raise ProviderUnavailableError(f"Could not complete task: {last_error}")

    def build_provider_chain(self, config):
        """Build chain based on config and availability"""
        primary = self.get_primary_provider(config)
        fallbacks = self.get_fallback_providers(config)
        last_resort = OllamaProvider()

        return [primary] + fallbacks + [last_resort]
```

### User Notification

```bash
# When primary provider fails
⚠️  OpenAI API unavailable
Falling back to Claude...

# When budget exceeded
⚠️  Daily OpenAI budget exceeded ($10.00)
Falling back to Ollama (free, local)

# When all providers fail
❌ All LLM providers unavailable
Some features will be limited:
- ❌ Project planning disabled
- ✅ Work update parsing (basic)
- ✅ Report generation (basic)

Reconnect to continue advanced features
```

---

## 🎯 Implementation Roadmap

### Phase 4: Hybrid LLM Foundation
```
Priority: High
Effort: 400 lines
Timeline: 2-3 weeks

Tasks:
├─ Enhanced provider factory
├─ Cost tracking system
├─ Configuration management
├─ Cost monitoring CLI
├─ Fallback strategy
└─ User documentation
```

### Phase 6: Smart Provider Selection
```
Priority: High
Effort: 300 lines
Timeline: 2 weeks

Tasks:
├─ Automatic provider selection
├─ Feature-specific providers
├─ Cost optimization
├─ Quality vs cost tradeoffs
└─ Dashboard integration
```

### Phase 8: Integration & Monitoring
```
Priority: Medium
Effort: 200 lines
Timeline: 1-2 weeks

Tasks:
├─ Web dashboard for usage
├─ Real-time cost alerts
├─ Provider health monitoring
├─ Performance metrics
└─ User analytics
```

---

## 🛡️ Security & Privacy

### API Key Management

```bash
# Secure storage (not in plain text)
$ devtrack llm set-api-key openai
Enter your OpenAI API key: [hidden input]
✓ Securely stored in ~/.devtrack/secrets.enc

# Verification without exposing
$ devtrack llm verify-api-key openai
✓ Valid OpenAI API key found

# Rotation
$ devtrack llm rotate-api-key openai
Enter new API key: [hidden input]
✓ API key rotated (old key revoked)
```

### Privacy Controls

```yaml
# Privacy-first configuration
llm:
  privacy:
    # Don't send certain data to cloud APIs
    never_send_to_api:
      - sensitive_projects
      - internal_codebases
      - employee_names
      - financial_data

    # For sensitive data, always use local
    sensitive_features:
      - knowledge_base_creation
      - internal_documentation
      - proprietary_analysis

    # Anonymize before sending
    anonymize_for_api:
      - project_names → "project_x"
      - file_names → "file_x"
      - code_snippets → [hash]
```

---

## 💡 Benefits of Hybrid Approach

### For Users

| Aspect | Offline Only | Hybrid | Cloud Only |
|--------|---|---|---|
| **Cost** | Free | Low (smart) | Higher |
| **Privacy** | Perfect | Good (configurable) | None |
| **Availability** | Always | Usually | Needs internet |
| **Quality** | Good | Excellent | Best |
| **Flexibility** | Fixed | Maximum | Fixed |

### For Developers

1. **Maximum Flexibility**
   - Users choose their comfort level
   - Mix and match providers
   - Optimize for their use case

2. **Graceful Degradation**
   - Always works (worst case: local)
   - No hard dependencies on APIs
   - Fallback chains ensure reliability

3. **Cost Optimization**
   - Expensive models for complex tasks
   - Cheap models for simple tasks
   - Free fallback always available

4. **Privacy by Default**
   - Local processing by default
   - APIs optional upgrade
   - User controls what leaves device

---

## 🎓 Example Scenarios

### Scenario 1: Student/Hobbyist
```yaml
llm:
  primary:
    provider: ollama
    model: mistral
  budget: $0/month

Result:
✓ Everything local
✓ Zero cost
✓ Works offline
✓ Good enough for learning
```

### Scenario 2: Startup Developer
```yaml
llm:
  primary:
    provider: openai
    model: gpt-3.5-turbo
  fallbacks:
    - ollama
  budget: $20/month

Result:
✓ Best bang for buck
✓ Good quality
✓ Cheap fallback
✓ Offline always works
```

### Scenario 3: Enterprise Developer
```yaml
llm:
  primary:
    provider: anthropic
    model: claude-3-opus
  fallbacks:
    - openai (gpt-4)
    - ollama (local)
  budget: $500/month
  privacy:
    never_send_to_api:
      - internal_projects
      - proprietary_code

Result:
✓ Best quality
✓ Privacy-first
✓ Multiple fallbacks
✓ Budget for reliability
```

---

## 📊 Estimated Costs

### Monthly Usage Examples

**Light User** (10 LLM calls/day)
```
OpenAI GPT-3.5: 5,000 tokens/day
Cost: 5,000 * 30 * $0.00050 = $0.75/month
```

**Medium User** (30 LLM calls/day)
```
OpenAI GPT-4: 10,000 tokens/day
Cost: 10,000 * 30 * $0.03 = $9.00/month
```

**Heavy User** (100+ LLM calls/day)
```
Claude Opus: 50,000 tokens/day
Cost: 50,000 * 30 * $0.015 = $22.50/month
```

All include free local Ollama as fallback.

---

## 🚀 Getting Started

### Setup Instructions

```bash
# 1. Install DevTrack (comes with Ollama setup)
git clone ...
cd automation_tools
uv sync

# 2. Test local Ollama
devtrack llm test-provider ollama
✓ Ollama working locally

# 3. (Optional) Add API provider
devtrack config set llm.primary.provider openai
devtrack config set llm.primary.api_key sk-...

# 4. Test fallback chain
devtrack llm test-provider openai
devtrack llm test-fallback  # Tests entire chain

# 5. Set cost limits (optional)
devtrack config set llm.cost_limits.daily 10.00
devtrack config set llm.cost_limits.monthly 200.00

# 6. Start using
devtrack start
```

---

## 📝 Summary

**Hybrid LLM Strategy** makes DevTrack:

✅ **Free by Default** - Ollama works offline
✅ **Optionally Better** - APIs for enhanced features
✅ **User Controlled** - Choose your quality/cost tradeoff
✅ **Always Available** - Graceful fallback chain
✅ **Privacy Friendly** - Local processing by default
✅ **Cost Optimized** - Smart provider selection
✅ **No Lock-in** - Works with any LLM provider

This makes DevTrack truly the Swiss Army knife:
- Works offline for anyone
- Upgradeable for those who want better
- Configurable for different needs
- Sustainable business model (optional premium features)

---

**Next Steps**: Implement Phase 4 with this hybrid architecture from day one!
