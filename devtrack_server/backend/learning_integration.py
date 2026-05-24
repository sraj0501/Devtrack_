"""
Integration module for personalized AI learning

This module integrates the personalized AI with existing MS Graph and Azure DevOps
implementations, making it easy to enable learning from the CLI.
"""

import asyncio
import json
import os
import sys
from pathlib import Path
from typing import Optional
from datetime import datetime, timezone

# Add paths
sys.path.insert(0, os.path.dirname(__file__))
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'msgraph_python'))
sys.path.insert(0, os.path.dirname(os.path.dirname(__file__)))  # repo root for backend.* imports

from personalized_ai import PersonalizedAI, CommunicationSample
from data_collectors import (
    TeamsDataCollector
)

try:
    from backend.db.mongo_learning import get_store as _get_mongo_store
    _mongo_import_ok = True
except ImportError:
    _mongo_import_ok = False


class GraphClientAdapter:
    """Adapter for the existing Graph client to work with data collectors"""
    
    def __init__(self, graph_client):
        self.graph = graph_client
    
    async def get_teams_chats(self):
        """Get all teams chats"""
        chats_response = await self.graph.get_teams_chats()
        if chats_response and hasattr(chats_response, 'value'):
            return [self._chat_to_dict(chat) for chat in chats_response.value]
        return []
    
    async def get_chat_messages(self, chat_id: str, since_datetime: Optional[datetime] = None):
        """Get messages from a specific chat"""
        messages_response = await self.graph.get_all_chat_messages(chat_id, since_datetime=since_datetime)
        return [self._message_to_dict(msg) for msg in messages_response]
    
    async def get_sent_emails(self, days: int = 30):
        """Get sent emails"""
        # Would use Graph API to get sent items
        # Placeholder for now
        return []
    
    def _chat_to_dict(self, chat):
        """Convert chat object to dictionary"""
        return {
            'id': chat.id if hasattr(chat, 'id') else None,
            'topic': chat.topic if hasattr(chat, 'topic') else None,
            'chatType': chat.chat_type if hasattr(chat, 'chat_type') else None,
            'createdDateTime': chat.created_date_time.isoformat() if hasattr(chat, 'created_date_time') and chat.created_date_time else None,
            'lastUpdatedDateTime': chat.last_updated_date_time.isoformat() if hasattr(chat, 'last_updated_date_time') and chat.last_updated_date_time else None
        }
    
    def _message_to_dict(self, message):
        """Convert message object to dictionary"""
        from_user = None
        if hasattr(message, 'from_') and message.from_:
            # Teams chat messages use from_.user (Identity), not from_.email_address
            if hasattr(message.from_, 'user') and message.from_.user:
                user_identity = message.from_.user
                # UPN is absent in Teams; use tenantId from additional_data if needed
                upn = None
                if hasattr(user_identity, 'additional_data') and user_identity.additional_data:
                    upn = user_identity.additional_data.get('userPrincipalName')
                display_name = getattr(user_identity, 'display_name', None)
                from_user = {
                    'user': {
                        'userPrincipalName': upn,
                        'displayName': display_name,
                        'id': getattr(user_identity, 'id', None),
                    }
                }
            # Fallback for Outlook-style messages with email_address
            elif hasattr(message.from_, 'email_address') and message.from_.email_address:
                from_user = {
                    'user': {
                        'userPrincipalName': getattr(message.from_.email_address, 'address', None),
                        'displayName': getattr(message.from_.email_address, 'name', None),
                    }
                }
        
        body_content = None
        if hasattr(message, 'body') and message.body:
            body_content = message.body.content if hasattr(message.body, 'content') else None
        
        return {
            'id': message.id if hasattr(message, 'id') else None,
            'createdDateTime': message.created_date_time.isoformat() if hasattr(message, 'created_date_time') and message.created_date_time else None,
            'from': from_user,
            'body': {
                'content': body_content
            }
        }


class AsyncTeamsDataCollector(TeamsDataCollector):
    """Async version of Teams data collector"""

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.user_object_id: Optional[str] = None

    def _is_user_message(self, message) -> bool:
        """Match by Azure AD object ID (more reliable than UPN for Teams messages)."""
        if not message:
            return False
        from_user = (message.get('from') or {}).get('user') or {}
        # Prefer ID match — UPN is often absent in Teams chat messages
        if self.user_object_id and from_user.get('id'):
            return from_user['id'] == self.user_object_id
        # Fallback to UPN
        return from_user.get('userPrincipalName', '') == self.ai.user_email

    async def collect_chat_history_async(
        self, days: int = 30, since_datetime: Optional[datetime] = None, mongo_store=None
    ) -> int:
        """Async version of collect_chat_history.

        Args:
            days: Fallback window if since_datetime not provided.
            since_datetime: Only collect messages newer than this timestamp.
        """
        if not self.ai.consent_given:
            print("⚠️  Consent not given. Cannot collect data.")
            return 0

        if not self.graph_client:
            print("❌ Graph client not initialized")
            return 0

        samples_collected = 0

        try:
            from datetime import timedelta

            # Get all chats
            chats = await self.graph_client.get_teams_chats()

            if since_datetime is not None:
                cutoff_date = since_datetime
                if cutoff_date.tzinfo is None:
                    cutoff_date = cutoff_date.replace(tzinfo=timezone.utc)
                print(f"📱 Collecting Teams messages since {since_datetime.strftime('%Y-%m-%d %H:%M')} (delta mode)...")
            else:
                cutoff_date = datetime.now(tz=timezone.utc) - timedelta(days=days)
                print(f"📱 Collecting Teams chat history for last {days} days...")
            
            for i, chat in enumerate(chats, 1):
                chat_id = chat.get('id')
                if not chat_id:
                    continue

                print(f"   Processing chat {i}/{len(chats)}...", end='\r')

                # Get messages from this chat (pass cutoff for early termination)
                messages = await self.graph_client.get_chat_messages(
                    chat_id, since_datetime=cutoff_date
                )

                if not messages or len(messages) < 2:
                    continue

                # Look for user responses (message following another person's message)
                for j in range(1, len(messages)):
                    prev_msg = messages[j-1]
                    current_msg = messages[j]

                    # Check if current message is from user
                    if self._is_user_message(current_msg):
                        # Check if previous message is NOT from user (someone asked/said something)
                        if not self._is_user_message(prev_msg):
                            trigger = self._extract_message_content(prev_msg)
                            response = self._extract_message_content(current_msg)

                            # Skip if either is empty
                            if not trigger or not response:
                                continue

                            msg_time = self._parse_timestamp(current_msg.get('createdDateTime', ''))
                            message_id = current_msg.get('id') or ''

                            if msg_time >= cutoff_date:
                                metadata = {
                                    'chat_id': chat_id,
                                    'timestamp': msg_time.isoformat(),
                                    'chat_type': chat.get('chatType', 'unknown'),
                                    'prev_message_id': prev_msg.get('id') or '',
                                }

                                # MongoDB upsert (deduplicated by message_id)
                                if mongo_store and mongo_store.is_available() and message_id:
                                    inserted = await mongo_store.upsert_sample(
                                        message_id=message_id,
                                        user_email=self.ai.user_email,
                                        source='teams',
                                        context_type='chat',
                                        trigger=trigger,
                                        response=response,
                                        timestamp=msg_time,
                                        metadata=metadata,
                                    )
                                    if not inserted:
                                        continue  # already stored, skip in-memory add too

                                # Add to in-memory list for profile computation
                                self.ai.add_communication_sample(
                                    source='teams',
                                    context_type='chat',
                                    trigger=trigger,
                                    response=response,
                                    metadata=metadata,
                                )
                                samples_collected += 1

            print(f"\n   ✓ Collected {samples_collected} Teams chat samples")
            return samples_collected
            
        except Exception as e:
            print(f"\n   ❌ Error collecting Teams data: {e}")
            import traceback
            traceback.print_exc()
            return samples_collected


def _load_last_collected() -> Optional[datetime]:
    """Load the last successful collection timestamp from SQLite."""
    try:
        # user_email may not be known at module level; read from DB by scanning all rows
        # and returning the most recent one — or None if table is empty.
        from backend.db.learning_store import _db_path
        import sqlite3
        con = sqlite3.connect(_db_path())
        row = con.execute(
            "SELECT last_collected FROM learning_sync_state ORDER BY last_collected DESC LIMIT 1"
        ).fetchone()
        con.close()
        if row and row[0]:
            dt = datetime.fromisoformat(row[0])
            if dt.tzinfo is None:
                dt = dt.replace(tzinfo=timezone.utc)
            return dt
    except Exception:
        pass
    return None


def _save_last_collected(ts: datetime, user_email: str = "") -> None:
    """Persist the collection timestamp for next delta run."""
    try:
        from backend.db.learning_store import save_last_collected
        save_last_collected(user_email or "default", ts)
    except Exception:
        pass


class LearningIntegration:
    """Main integration class for personalized AI learning"""

    def __init__(self, user_email: Optional[str] = None):
        """
        Initialize learning integration

        Args:
            user_email: User's email address (will try to fetch if not provided)
        """
        self.user_email = user_email
        self.user_object_id: Optional[str] = None
        self.ai = None
        self.graph_adapter = None
        self.teams_collector = None
        self._mongo = _get_mongo_store() if _mongo_import_ok else None
    
    async def initialize(self, graph_client=None):
        """
        Initialize with MS Graph client
        
        Args:
            graph_client: Instance of msgraph_python.graph.Graph
        """
        # Get user email + Azure AD object ID
        if graph_client:
            try:
                user = await graph_client.get_user()
                if not self.user_email:
                    self.user_email = user.mail or user.user_principal_name
                    print(f"📧 Using email: {self.user_email}")
                self.user_object_id = getattr(user, 'id', None)
                # Persist user_object_id to DB so future runs can use it without re-auth
                try:
                    from backend.db.learning_store import update_user_object_id
                    if self.user_email and self.user_object_id:
                        update_user_object_id(self.user_email, self.user_object_id)
                except Exception:
                    pass
            except Exception as e:
                print(f"⚠️  Could not get user info from Graph: {e}")
                # Load from DB (persisted from previous successful auth)
                try:
                    from backend.db.learning_store import load_consent
                    if self.user_email:
                        record = load_consent(self.user_email)
                        if record:
                            self.user_object_id = record.get("user_object_id")
                except Exception:
                    pass
                # Fall back to env var if still empty
                if not self.user_email:
                    try:
                        from backend.config import get
                        self.user_email = get("EMAIL", "") or get("USER_EMAIL", "")
                    except ImportError:
                        self.user_email = ""
        
        # Initialize PersonalizedAI
        self.ai = PersonalizedAI(self.user_email)

        # Enable MongoDB mode if available (suppresses redundant file writes)
        if self._mongo and self._mongo.is_available():
            self.ai._mongo_mode = True
            await self._mongo.ensure_indexes()
            # Seed in-memory samples from MongoDB for profile computation
            try:
                stored = await self._mongo.load_samples_for_profile(self.user_email)
                if stored:
                    from personalized_ai import CommunicationSample
                    for s in stored:
                        ts = s.get('timestamp')
                        if ts and isinstance(ts, str):
                            ts = datetime.fromisoformat(ts)
                        elif ts is None:
                            ts = datetime.now(timezone.utc)
                        sample = CommunicationSample(
                            id=str(s.get('_id', '')),
                            source=s.get('source', ''),
                            timestamp=ts,
                            context_type=s.get('context_type', ''),
                            trigger=s.get('trigger', ''),
                            response=s.get('response', ''),
                            metadata=s.get('metadata', {}),
                        )
                        self.ai.samples.append(sample)
                    if stored:
                        self.ai._update_profile()
                        print(f"   ℹ️  Loaded {len(stored)} existing samples from MongoDB")
            except Exception as e:
                print(f"   ⚠️  Could not seed samples from MongoDB: {e}")

        # Initialize collectors
        if graph_client:
            self.graph_adapter = GraphClientAdapter(graph_client)
            self.teams_collector = AsyncTeamsDataCollector(self.ai)
            self.teams_collector.graph_client = self.graph_adapter
            self.teams_collector.user_object_id = self.user_object_id
    
    async def collect_teams_data(self, days: int = 30, force_full: bool = False) -> int:
        """
        Collect data from MS Teams (delta mode by default).

        On first run, collects the last `days` days of history.
        On subsequent runs, only collects messages since the last successful run.

        Args:
            days: Initial history window (used only when no prior state exists).
            force_full: If True, ignore stored state and re-collect last `days` days.

        Returns:
            Number of new samples collected.
        """
        if not self.ai.consent_given:
            print("\n⚠️  Learning consent not given.")
            print("    The AI needs your permission to learn from your communications.")
            print()
            response = input("Would you like to grant consent? (yes/no): ").strip().lower()

            if response in ['yes', 'y']:
                print()
                self.ai.request_consent()
                print()
            else:
                print("\n❌ Cannot collect data without consent.")
                return 0

        if not self.teams_collector:
            print("❌ Teams collector not initialized")
            return 0

        # Determine whether to use delta or full fetch
        # MongoDB takes precedence over file-based state
        since_datetime = None
        if not force_full:
            if self._mongo and self._mongo.is_available():
                since_datetime = await self._mongo.load_last_collected(self.user_email)
            if since_datetime is None:
                since_datetime = _load_last_collected()  # file fallback

        print()
        print("=" * 70)
        print("📥 COLLECTING COMMUNICATION DATA")
        if self._mongo and self._mongo.is_available():
            print("   Storage: MongoDB")
        else:
            print("   Storage: files (set MONGODB_URI in .env for MongoDB)")
        print("=" * 70)
        print()

        collection_start = datetime.now(tz=timezone.utc)
        count = await self.teams_collector.collect_chat_history_async(
            days=days, since_datetime=since_datetime, mongo_store=self._mongo
        )

        if count > 0:
            print()
            print("🧠 Updating AI profile with new data...")
            self.ai._update_profile()
            # Persist profile to MongoDB
            if self._mongo and self._mongo.is_available() and self.ai.profile:
                await self._mongo.upsert_profile(self.user_email, self.ai.profile)
            print("   ✓ Profile updated")
            print()
            print("=" * 70)
            print(f"✅ Successfully collected and processed {count} samples")
            print("=" * 70)

        # Persist delta state — MongoDB first, SQLite as fallback
        if self._mongo and self._mongo.is_available():
            await self._mongo.save_last_collected(self.user_email, collection_start)
        _save_last_collected(collection_start, user_email=self.user_email or "")

        return count
    
    def show_profile(self):
        """Display the current learning profile"""
        if not self.ai:
            print("❌ AI not initialized")
            return
        
        print()
        print(self.ai.get_profile_summary())
    
    async def test_response_generation(self, trigger: str, context: str = "work"):
        """
        Test generating a response for a given trigger
        
        Args:
            trigger: The message/question to respond to
            context: Context type (work, casual, formal)
        """
        if not self.ai:
            print("❌ AI not initialized")
            return
        
        if not self.ai.consent_given:
            print("⚠️  Consent not given. Cannot generate responses.")
            return
        
        print()
        print("=" * 70)
        print("🤖 GENERATING PERSONALIZED RESPONSE")
        print("=" * 70)
        print()
        print(f"Trigger: {trigger}")
        print(f"Context: {context}")
        print()
        print("Generating...")
        print()
        
        response = self.ai.generate_response_suggestion(trigger, context)
        
        print("Suggested Response:")
        print("-" * 70)
        print(response)
        print("-" * 70)
        print()


async def main():
    """CLI interface for learning integration"""
    import sys
    from configparser import ConfigParser
    from msgraph_python.graph import Graph
    
    # Load config
    config = ConfigParser()
    config_path = os.path.join(os.path.dirname(__file__), 'msgraph_python', 'config.cfg')
    
    if not os.path.exists(config_path):
        print(f"❌ Config file not found: {config_path}")
        print("   Please set up MS Graph authentication first.")
        return
    
    config.read(config_path)
    
    if len(sys.argv) < 2:
        print("Usage: python learning_integration.py <command> [options]")
        print()
        print("Commands:")
        print("  enable-learning [days]  - Start collecting data and enable learning")
        print("  show-profile           - Show current learning profile")
        print("  test-response <text>   - Test generating a response")
        print("  revoke-consent         - Revoke learning consent and delete data")
        print()
        return
    
    command = sys.argv[1]

    # Commands that don't need Graph API auth
    no_graph_commands = {'test-response', 'show-profile', 'revoke-consent'}

    if command in no_graph_commands:
        integration = LearningIntegration()
        await integration.initialize(graph_client=None)
    else:
        # Initialize Graph client for commands that need it
        print("🔐 Authenticating with Microsoft Graph...")
        graph = Graph(config['azure'])
        integration = LearningIntegration()
        await integration.initialize(graph)
    
    if command == "enable-learning":
        days = int(sys.argv[2]) if len(sys.argv) > 2 else 30
        await integration.collect_teams_data(days)
        integration.show_profile()
    
    elif command == "show-profile":
        integration.show_profile()
    
    elif command == "test-response":
        if len(sys.argv) < 3:
            print("❌ Please provide text to respond to")
            return
        
        trigger = " ".join(sys.argv[2:])
        await integration.test_response_generation(trigger)
    
    elif command == "revoke-consent":
        if integration.ai:
            print()
            confirm = input("⚠️  This will delete all learned data. Continue? (yes/no): ").strip().lower()
            if confirm in ['yes', 'y']:
                delete_data = input("Also delete all collected samples? (yes/no): ").strip().lower()
                integration.ai.revoke_consent(delete_data=(delete_data in ['yes', 'y']))
            else:
                print("Cancelled.")
    
    else:
        print(f"❌ Unknown command: {command}")


if __name__ == "__main__":
    asyncio.run(main())
