---
name: Wiki GIF policy
description: When to add GIFs to the wiki/marketing site — don't add until the feature actually works end-to-end
type: feedback
---

Do not add VHS-generated GIFs (or any demo GIFs) to the wiki site unless the feature they demonstrate is actually working and can be replicated by the user.

**Why:** Placeholder or simulated GIFs add no value to a sales page. If a visitor tries to replicate what they see and it doesn't work, trust is destroyed. The JS typewriter terminal in the hero is fine because it's clearly an animation — a GIF implies a real recording.

**How to apply:** VHS `.tape` files live in `wiki/tapes/` and generated GIFs in `wiki/assets/` — keep them there for when features are ready. Wire them into the HTML only when the recorded workflow is actually functional. The planned home for real demo GIFs is a future Tutorial section, not the hero or "Day in the Life" section.
