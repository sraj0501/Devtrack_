"""
Personalized Communication AI Module

This module learns from the user's communication history across MS Teams, 
Azure DevOps, and Outlook to understand their communication style and 
eventually assist with personalized responses.

Key Features:
- Collects communication data with explicit user consent
- Analyzes writing style, tone, and response patterns
- Learns context-specific communication patterns
- Uses Ollama for local, privacy-preserving AI training
- Provides personalized response suggestions
- Continuously improves over time

Privacy: All data processing happens locally. No data leaves the user's machine.
"""

import os
import json
import logging
from pathlib import Path
from datetime import datetime
from typing import Dict, List, Optional
from dataclasses import dataclass, asdict
from collections import defaultdict
import re


logger = logging.getLogger(__name__)


@dataclass
class CommunicationSample:
    """Represents a single communication sample for learning"""
    id: str
    source: str  # "teams", "azure_devops", "outlook"
    timestamp: datetime
    context_type: str  # "email", "chat", "comment", "meeting"
    trigger: str  # What prompted the response
    response: str  # User's actual response
    metadata: Dict
    
    def to_dict(self) -> Dict:
        return {
            **asdict(self),
            'timestamp': self.timestamp.isoformat()
        }


@dataclass
class CommunicationPattern:
    """Represents learned communication patterns"""
    context_type: str
    common_phrases: List[str]
    avg_response_length: int
    tone: str  # "formal", "casual", "technical", etc.
    response_time: str  # "immediate", "hours", "days"
    typical_structure: List[str]
    sentiment_distribution: Dict[str, float]


@dataclass
class UserProfile:
    """User's communication profile"""
    user_email: str
    writing_style: Dict[str, any]
    response_patterns: Dict[str, CommunicationPattern]
    vocabulary: Dict[str, int]  # Word frequency
    common_sign_offs: List[str]
    common_greetings: List[str]
    preferred_pronouns: List[str]
    last_updated: datetime
    total_samples: int


class PersonalizedAI:
    """
    Personalized AI that learns from user's communication history
    """
    
    def __init__(self, user_email: str, data_dir: str = None):
        """
        Initialize personalized AI
        
        Args:
            user_email: User's email address
            data_dir: Directory to store learning data (default: ~/.devtrack/learning)
        """
        self.user_email = user_email
        
        # Setup data directory - use config, else derive from project root
        if data_dir is None:
            try:
                from backend.config import learning_dir
                data_dir = str(learning_dir())
            except ImportError:
                try:
                    from backend.utils.paths import fallback_path
                    data_dir = fallback_path("Data", "learning")
                except ImportError:
                    _cur = Path(__file__).resolve().parent.parent
                    data_dir = str(_cur / "Data" / "learning")
        
        self.data_dir = data_dir
        os.makedirs(self.data_dir, exist_ok=True)
        
        # Storage files
        self.samples_file = os.path.join(data_dir, "communication_samples.jsonl")
        self.profile_file = os.path.join(data_dir, "user_profile.json")
        self.consent_file = os.path.join(data_dir, "consent.json")
        
        # Load existing data
        self.samples: List[CommunicationSample] = []
        self.profile: Optional[UserProfile] = None
        self.consent_given = self._check_consent()
        # When True, skip JSONL file writes (MongoDB is the primary store)
        self._mongo_mode: bool = False
        
        if self.consent_given:
            self._load_samples()
            self._load_profile()
        
        logger.info(f"PersonalizedAI initialized for {user_email}")
        logger.info(f"Consent given: {self.consent_given}")
        logger.info(f"Samples loaded: {len(self.samples)}")
    
    def _check_consent(self) -> bool:
        """Check if user has given explicit consent"""
        if not os.path.exists(self.consent_file):
            return False
        
        try:
            with open(self.consent_file, 'r') as f:
                consent = json.load(f)
                return consent.get('consent_given', False)
        except:
            return False
    
    def request_consent(self) -> bool:
        """
        Request user consent for data collection and learning
        
        Returns:
            True if consent given
        """
        print("\n" + "="*70)
        print("🔒 PERSONALIZED AI - CONSENT REQUEST")
        print("="*70)
        print()
        print("The DevTrack system can learn from your communication patterns to")
        print("provide personalized assistance. This feature:")
        print()
        print("✓ Analyzes your writing style across MS Teams, Azure DevOps, and Outlook")
        print("✓ Learns how you respond in different situations")
        print("✓ Can suggest responses or drafts in your style")
        print("✓ Improves over time as it learns more about your preferences")
        print()
        print("🔐 PRIVACY GUARANTEES:")
        print("   • All data stays on YOUR machine (local storage only)")
        print("   • Uses Ollama - local AI, no cloud services")
        print("   • You can revoke consent and delete all data anytime")
        print("   • No data is sent to external servers")
        print("   • You control what sources are monitored")
        print()
        print("📊 DATA COLLECTED:")
        print("   • Your messages and responses (Teams, Email, DevOps comments)")
        print("   • Timestamps and context (but not personal info of others)")
        print("   • Writing patterns and vocabulary")
        print()
        
        response = input("Do you consent to this data collection and learning? (yes/no): ").strip().lower()
        
        if response in ['yes', 'y']:
            consent_data = {
                'consent_given': True,
                'timestamp': datetime.now().isoformat(),
                'user_email': self.user_email,
                'version': '1.0',
                'features': [
                    'communication_learning',
                    'style_analysis',
                    'response_suggestions'
                ]
            }
            
            with open(self.consent_file, 'w') as f:
                json.dump(consent_data, f, indent=2)
            
            self.consent_given = True
            print()
            print("✅ Consent recorded. The AI will now start learning from your communications.")
            print(f"   Data stored in: {self.data_dir}")
            print()
            return True
        else:
            print()
            print("❌ Consent not given. Personalized AI features will remain disabled.")
            print("   You can enable this later by running: devtrack enable-learning")
            print()
            return False
    
    def revoke_consent(self):
        """Revoke consent and optionally delete all collected data"""
        if not self.consent_given:
            print("Consent was not previously given.")
            return
        
        print("\n🔒 REVOKING CONSENT")
        print("="*70)
        delete = input("Do you want to DELETE all collected data? (yes/no): ").strip().lower()
        
        if delete in ['yes', 'y']:
            # Delete all data
            for file in [self.samples_file, self.profile_file, self.consent_file]:
                if os.path.exists(file):
                    os.remove(file)
            
            self.samples = []
            self.profile = None
            self.consent_given = False
            
            print("✅ Consent revoked and all data deleted.")
        else:
            # Just revoke consent, keep data
            consent_data = {
                'consent_given': False,
                'revoked_at': datetime.now().isoformat(),
                'user_email': self.user_email
            }
            
            with open(self.consent_file, 'w') as f:
                json.dump(consent_data, f, indent=2)
            
            self.consent_given = False
            print("✅ Consent revoked. Data preserved but AI learning disabled.")
    
    def _load_samples(self):
        """Load communication samples from disk"""
        if not os.path.exists(self.samples_file):
            return
        
        try:
            with open(self.samples_file, 'r') as f:
                for line in f:
                    if line.strip():
                        data = json.loads(line)
                        data['timestamp'] = datetime.fromisoformat(data['timestamp'])
                        sample = CommunicationSample(**data)
                        self.samples.append(sample)
            
            logger.info(f"Loaded {len(self.samples)} communication samples")
        except Exception as e:
            logger.error(f"Error loading samples: {e}")
    
    def _load_profile(self):
        """Load user profile from disk"""
        if not os.path.exists(self.profile_file):
            return
        
        try:
            with open(self.profile_file, 'r') as f:
                data = json.load(f)
                data['last_updated'] = datetime.fromisoformat(data['last_updated'])
                
                # Convert response_patterns back to objects
                patterns = {}
                for key, pattern_data in data.get('response_patterns', {}).items():
                    patterns[key] = CommunicationPattern(**pattern_data)
                data['response_patterns'] = patterns
                
                self.profile = UserProfile(**data)
            
            logger.info(f"Loaded user profile: {self.profile.total_samples} samples analyzed")
        except Exception as e:
            logger.error(f"Error loading profile: {e}")
    
    def _save_sample(self, sample: CommunicationSample):
        """Save a communication sample to file (skipped in MongoDB mode)."""
        if self._mongo_mode:
            return
        with open(self.samples_file, 'a') as f:
            f.write(json.dumps(sample.to_dict()) + '\n')
    
    def _save_profile(self):
        """Save user profile to file (skipped in MongoDB mode)."""
        if self._mongo_mode or not self.profile:
            return
        
        profile_data = asdict(self.profile)
        profile_data['last_updated'] = self.profile.last_updated.isoformat()
        
        # Convert CommunicationPattern objects
        patterns = {}
        for key, pattern in self.profile.response_patterns.items():
            patterns[key] = asdict(pattern)
        profile_data['response_patterns'] = patterns
        
        with open(self.profile_file, 'w') as f:
            json.dump(profile_data, f, indent=2)
    
    def add_communication_sample(
        self,
        source: str,
        context_type: str,
        trigger: str,
        response: str,
        metadata: Dict = None
    ) -> bool:
        """
        Add a new communication sample for learning
        
        Args:
            source: "teams", "azure_devops", "outlook"
            context_type: "email", "chat", "comment", "meeting"
            trigger: What prompted the response
            response: User's actual response
            metadata: Additional context
            
        Returns:
            True if sample was added
        """
        if not self.consent_given:
            logger.warning("Consent not given. Cannot add sample.")
            return False
        
        sample = CommunicationSample(
            id=f"{source}_{datetime.now().timestamp()}",
            source=source,
            timestamp=datetime.now(),
            context_type=context_type,
            trigger=trigger,
            response=response,
            metadata=metadata or {}
        )
        
        self.samples.append(sample)
        self._save_sample(sample)
        
        logger.info(f"Added communication sample from {source}/{context_type}")
        
        # Trigger learning update if we have enough samples
        if len(self.samples) % 10 == 0:
            self._update_profile()
        
        return True
    
    def _update_profile(self):
        """Update user profile based on collected samples"""
        logger.info("Updating user profile from samples...")
        
        # Analyze all samples
        writing_style = self._analyze_writing_style()
        response_patterns = self._analyze_response_patterns()
        vocabulary = self._analyze_vocabulary()
        sign_offs = self._extract_sign_offs()
        greetings = self._extract_greetings()
        
        self.profile = UserProfile(
            user_email=self.user_email,
            writing_style=writing_style,
            response_patterns=response_patterns,
            vocabulary=vocabulary,
            common_sign_offs=sign_offs,
            common_greetings=greetings,
            preferred_pronouns=self._detect_pronouns(),
            last_updated=datetime.now(),
            total_samples=len(self.samples)
        )
        
        self._save_profile()
        logger.info("Profile updated successfully")
    
    def _analyze_writing_style(self) -> Dict:
        """Analyze user's writing style"""
        if not self.samples:
            return {}
        
        total_length = 0
        sentence_count = 0
        word_count = 0
        
        for sample in self.samples:
            text = sample.response
            total_length += len(text)
            sentences = text.split('.')
            sentence_count += len(sentences)
            word_count += len(text.split())
        
        avg_response_length = total_length / len(self.samples)
        avg_sentence_length = word_count / max(sentence_count, 1)
        
        return {
            'avg_response_length': avg_response_length,
            'avg_sentence_length': avg_sentence_length,
            'avg_word_count': word_count / len(self.samples),
            'uses_emojis': self._check_emoji_usage(),
            'formality_level': self._estimate_formality(),
        }
    
    def _analyze_response_patterns(self) -> Dict[str, CommunicationPattern]:
        """Analyze response patterns by context type"""
        patterns = {}
        
        # Group samples by context type
        by_context = defaultdict(list)
        for sample in self.samples:
            by_context[sample.context_type].append(sample)
        
        # Analyze each context type
        for context_type, samples in by_context.items():
            phrases = self._extract_common_phrases(samples)
            avg_length = sum(len(s.response) for s in samples) / len(samples)
            
            pattern = CommunicationPattern(
                context_type=context_type,
                common_phrases=phrases[:20],  # Top 20
                avg_response_length=int(avg_length),
                tone=self._detect_tone(samples),
                response_time=self._analyze_response_time(samples),
                typical_structure=self._analyze_structure(samples),
                sentiment_distribution={}
            )
            
            patterns[context_type] = pattern
        
        return patterns
    
    def _analyze_vocabulary(self) -> Dict[str, int]:
        """Build vocabulary frequency map"""
        vocabulary = defaultdict(int)
        
        for sample in self.samples:
            words = re.findall(r'\b\w+\b', sample.response.lower())
            for word in words:
                if len(word) > 3:  # Skip very short words
                    vocabulary[word] += 1
        
        # Return top 500 words
        sorted_vocab = sorted(vocabulary.items(), key=lambda x: x[1], reverse=True)
        return dict(sorted_vocab[:500])
    
    def _extract_sign_offs(self) -> List[str]:
        """Extract common sign-off phrases"""
        sign_offs = defaultdict(int)
        
        patterns = [
            r'(thanks?\s*,?.*?)$',
            r'(regards?\s*,?.*?)$',
            r'(best\s*,?.*?)$',
            r'(cheers?\s*,?.*?)$',
        ]
        
        for sample in self.samples:
            text = sample.response.lower().strip()
            for pattern in patterns:
                match = re.search(pattern, text, re.IGNORECASE | re.MULTILINE)
                if match:
                    sign_offs[match.group(1)] += 1
        
        sorted_sign_offs = sorted(sign_offs.items(), key=lambda x: x[1], reverse=True)
        return [s[0] for s in sorted_sign_offs[:10]]
    
    def _extract_greetings(self) -> List[str]:
        """Extract common greeting phrases"""
        greetings = defaultdict(int)
        
        patterns = [
            r'^(hi\s*,?.*?[.!])',
            r'^(hello\s*,?.*?[.!])',
            r'^(hey\s*,?.*?[.!])',
            r'^(good\s+(?:morning|afternoon|evening)\s*,?.*?[.!])',
        ]
        
        for sample in self.samples:
            text = sample.response.strip()
            for pattern in patterns:
                match = re.search(pattern, text, re.IGNORECASE)
                if match:
                    greetings[match.group(1)] += 1
        
        sorted_greetings = sorted(greetings.items(), key=lambda x: x[1], reverse=True)
        return [g[0] for g in sorted_greetings[:10]]
    
    def _detect_pronouns(self) -> List[str]:
        """Detect preferred pronouns"""
        pronouns = defaultdict(int)
        
        for sample in self.samples:
            text = sample.response.lower()
            if ' i ' in text or text.startswith('i '):
                pronouns['I'] += 1
            if ' we ' in text or text.startswith('we '):
                pronouns['we'] += 1
        
        return [p for p, _ in sorted(pronouns.items(), key=lambda x: x[1], reverse=True)]
    
    def _check_emoji_usage(self) -> bool:
        """Check if user commonly uses emojis"""
        emoji_count = 0
        emoji_pattern = re.compile(r'[\U0001F600-\U0001F64F\U0001F300-\U0001F5FF]')
        
        for sample in self.samples:
            if emoji_pattern.search(sample.response):
                emoji_count += 1
        
        return emoji_count > len(self.samples) * 0.1  # More than 10% use emojis
    
    def _estimate_formality(self) -> str:
        """Estimate formality level"""
        formal_indicators = ['please', 'kindly', 'appreciate', 'regards', 'sincerely']
        casual_indicators = ['hey', 'thanks', 'cool', 'awesome', 'yeah']
        
        formal_score = 0
        casual_score = 0
        
        for sample in self.samples:
            text = sample.response.lower()
            for indicator in formal_indicators:
                if indicator in text:
                    formal_score += 1
            for indicator in casual_indicators:
                if indicator in text:
                    casual_score += 1
        
        if formal_score > casual_score * 1.5:
            return 'formal'
        elif casual_score > formal_score * 1.5:
            return 'casual'
        else:
            return 'balanced'
    
    def _extract_common_phrases(self, samples: List[CommunicationSample]) -> List[str]:
        """Extract common multi-word phrases"""
        phrases = defaultdict(int)
        
        for sample in samples:
            words = sample.response.lower().split()
            # Extract 2-3 word phrases
            for i in range(len(words) - 1):
                bigram = ' '.join(words[i:i+2])
                phrases[bigram] += 1
                if i < len(words) - 2:
                    trigram = ' '.join(words[i:i+3])
                    phrases[trigram] += 1
        
        sorted_phrases = sorted(phrases.items(), key=lambda x: x[1], reverse=True)
        return [p[0] for p in sorted_phrases if p[1] > 1][:20]
    
    def _detect_tone(self, samples: List[CommunicationSample]) -> str:
        """Detect overall tone"""
        # Simplified tone detection
        positive_words = {'thanks', 'great', 'good', 'excellent', 'appreciate', 'happy'}
        negative_words = {'issue', 'problem', 'bug', 'error', 'concern', 'unfortunately'}
        
        positive_count = 0
        negative_count = 0
        
        for sample in samples:
            words = set(sample.response.lower().split())
            positive_count += len(words & positive_words)
            negative_count += len(words & negative_words)
        
        if positive_count > negative_count * 1.5:
            return 'positive'
        elif negative_count > positive_count:
            return 'professional'
        else:
            return 'neutral'
    
    def _analyze_response_time(self, samples: List[CommunicationSample]) -> str:
        """Analyze typical response time (placeholder)"""
        # This would analyze timestamp differences in real implementation
        return 'varies'
    
    def _analyze_structure(self, samples: List[CommunicationSample]) -> List[str]:
        """Analyze typical response structure"""
        structures = []
        
        for sample in samples[:5]:  # Analyze first 5
            text = sample.response
            structure = []
            
            if re.match(r'^(hi|hello|hey)', text, re.IGNORECASE):
                structure.append('greeting')
            if '?' in text:
                structure.append('question')
            if len(text.split('\n')) > 1:
                structure.append('multi_paragraph')
            if re.search(r'(thanks|regards|best)', text, re.IGNORECASE):
                structure.append('sign_off')
            
            structures.extend(structure)
        
        return list(set(structures))
    
    def generate_response_suggestion(
        self,
        context_type: str,
        trigger: str,
        additional_context: str = ""
    ) -> str:
        """
        Generate a personalized response suggestion using Ollama
        
        Args:
            context_type: Type of communication ("email", "chat", etc.)
            trigger: What prompted the response
            additional_context: Additional context for the response
            
        Returns:
            Suggested response in user's style
        """
        if not self.consent_given:
            return "Error: Consent not given for personalized AI"
        
        if not self.profile:
            return "Error: Not enough data to generate personalized responses yet"
        
        # Build prompt with user's style characteristics
        style_info = self._build_style_prompt()
        
        prompt = f"""You are writing a response for a user with the following communication style:

{style_info}

Context Type: {context_type}
Trigger: {trigger}
Additional Context: {additional_context}

Generate a response that matches this user's style, tone, and typical structure. 
Use their common phrases and vocabulary where appropriate.

Response:"""
        
        try:
            from backend.llm import get_provider
            from backend.llm.base import LLMOptions
            from backend.config import personalization_llm_temperature, personalization_llm_max_tokens
            result = get_provider().generate(
                prompt=prompt,
                options=LLMOptions(
                    temperature=personalization_llm_temperature(),
                    max_tokens=personalization_llm_max_tokens(),
                ),
            )
            return result if result else "Error: LLM unavailable"
        except Exception as e:
            logger.error(f"Error generating response: {e}")
            return f"Error generating response: {e}"
    
    def _build_style_prompt(self) -> str:
        """Build a prompt describing user's communication style"""
        if not self.profile:
            return "No style information available"
        
        style = self.profile.writing_style
        
        prompt_parts = [
            f"- Formality: {style.get('formality_level', 'balanced')}",
            f"- Average response length: {style.get('avg_word_count', 50):.0f} words",
            f"- Uses emojis: {'yes' if style.get('uses_emojis') else 'no'}",
        ]
        
        if self.profile.common_greetings:
            prompt_parts.append(f"- Common greetings: {', '.join(self.profile.common_greetings[:3])}")
        
        if self.profile.common_sign_offs:
            prompt_parts.append(f"- Common sign-offs: {', '.join(self.profile.common_sign_offs[:3])}")
        
        # Add context-specific patterns
        if context_type := self.profile.response_patterns.get('email'):
            prompt_parts.append(f"- Email tone: {context_type.tone}")
            if context_type.common_phrases:
                prompt_parts.append(f"- Frequently uses: {', '.join(context_type.common_phrases[:5])}")
        
        return '\n'.join(prompt_parts)
    
    def get_profile_summary(self) -> str:
        """Get a summary of the learned profile"""
        if not self.consent_given:
            return "Consent not given. Personalized AI disabled."
        
        if not self.profile:
            return "No profile data yet. Start by collecting communication samples."
        
        summary = f"""
📊 PERSONALIZED AI PROFILE SUMMARY
{'='*70}

User: {self.profile.user_email}
Total Samples: {self.profile.total_samples}
Last Updated: {self.profile.last_updated.strftime('%Y-%m-%d %H:%M')}

WRITING STYLE:
  Formality: {self.profile.writing_style.get('formality_level', 'balanced')}
  Avg Response: {self.profile.writing_style.get('avg_word_count', 0):.0f} words
  Avg Sentence: {self.profile.writing_style.get('avg_sentence_length', 0):.0f} words
  Uses Emojis: {'Yes' if self.profile.writing_style.get('uses_emojis') else 'No'}

COMMON PHRASES:
  Greetings: {', '.join(self.profile.common_greetings[:3]) if self.profile.common_greetings else 'None yet'}
  Sign-offs: {', '.join(self.profile.common_sign_offs[:3]) if self.profile.common_sign_offs else 'None yet'}

CONTEXT PATTERNS:
"""
        
        for context, pattern in self.profile.response_patterns.items():
            summary += f"  {context.upper()}: {pattern.tone} tone, ~{pattern.avg_response_length} chars\n"
        
        summary += f"\nTOP VOCABULARY: {', '.join(list(self.profile.vocabulary.keys())[:10])}\n"
        
        return summary


# CLI interface for personalized AI management
if __name__ == "__main__":
    import sys
    
    if len(sys.argv) < 2:
        print("Usage: python personalized_ai.py <command> [user_email]")
        print("Commands: request-consent, revoke-consent, show-profile")
        sys.exit(1)
    
    command = sys.argv[1]
    user_email = sys.argv[2] if len(sys.argv) > 2 else "user@example.com"
    
    ai = PersonalizedAI(user_email)
    
    if command == "request-consent":
        ai.request_consent()
    elif command == "revoke-consent":
        ai.revoke_consent()
    elif command == "show-profile":
        print(ai.get_profile_summary())
    else:
        print(f"Unknown command: {command}")
