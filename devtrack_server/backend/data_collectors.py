"""
Communication Data Collectors

This module provides data collectors for MS Teams, Azure DevOps, and Outlook
to gather communication samples for the personalized AI learning system.

All data collection requires explicit user consent and happens locally.
"""

import os
import logging
from datetime import datetime, timedelta
from typing import List, Dict
import sys

# Add backend to path
sys.path.insert(0, os.path.dirname(__file__))

from personalized_ai import PersonalizedAI

logger = logging.getLogger(__name__)


class TeamsDataCollector:
    """Collect communication data from MS Teams"""
    
    def __init__(self, personalized_ai: PersonalizedAI):
        self.ai = personalized_ai
        self.graph_client = None
    
    def initialize(self, graph_client):
        """Initialize with Microsoft Graph client"""
        self.graph_client = graph_client
        logger.info("Teams data collector initialized")
    
    def collect_chat_history(self, days: int = 30) -> int:
        """
        Collect chat history from Teams
        
        Args:
            days: Number of days of history to collect
            
        Returns:
            Number of samples collected
        """
        if not self.ai.consent_given:
            logger.warning("Consent not given. Cannot collect data.")
            return 0
        
        if not self.graph_client:
            logger.error("Graph client not initialized")
            return 0
        
        logger.info(f"Collecting Teams chat history for last {days} days...")
        samples_collected = 0
        
        try:
            # Get all chats
            chats = self.graph_client.get_teams_chats()
            
            cutoff_date = datetime.now() - timedelta(days=days)
            
            for chat in chats:
                chat_id = chat.get('id')
                
                # Get messages from this chat
                messages = self._get_chat_messages(chat_id)
                
                for i in range(1, len(messages)):
                    prev_msg = messages[i-1]
                    current_msg = messages[i]
                    
                    # Check if current message is from user (response)
                    if self._is_user_message(current_msg):
                        # Previous message is the trigger
                        trigger = self._extract_message_content(prev_msg)
                        response = self._extract_message_content(current_msg)
                        
                        msg_time = self._parse_timestamp(current_msg.get('createdDateTime'))
                        
                        if msg_time >= cutoff_date:
                            self.ai.add_communication_sample(
                                source='teams',
                                context_type='chat',
                                trigger=trigger,
                                response=response,
                                metadata={
                                    'chat_id': chat_id,
                                    'timestamp': msg_time.isoformat(),
                                    'chat_type': chat.get('chatType', 'unknown')
                                }
                            )
                            samples_collected += 1
            
            logger.info(f"Collected {samples_collected} Teams chat samples")
            return samples_collected
            
        except Exception as e:
            logger.error(f"Error collecting Teams data: {e}")
            return samples_collected
    
    def _get_chat_messages(self, chat_id: str) -> List[Dict]:
        """Get messages from a specific chat"""
        # Implementation depends on graph client
        # This is a placeholder
        try:
            # Would call something like:
            # return self.graph_client.get_chat_messages(chat_id)
            return []
        except Exception as e:
            logger.error(f"Error getting chat messages: {e}")
            return []
    
    def _is_user_message(self, message: Dict) -> bool:
        """Check if message is from the user"""
        if not message:
            return False
        from_field = message.get('from') or {}
        from_user = from_field.get('user') or {}
        user_email = from_user.get('userPrincipalName', '')
        return user_email == self.ai.user_email
    
    def _extract_message_content(self, message: Dict) -> str:
        """Extract clean message content"""
        if not message:
            return ''
        body = message.get('body') or {}
        content = body.get('content', '')
        
        # Remove HTML tags if present
        import re
        content = re.sub(r'<[^>]+>', '', content)
        
        return content.strip()
    
    def _parse_timestamp(self, timestamp_str: str) -> datetime:
        """Parse ISO timestamp"""
        try:
            return datetime.fromisoformat(timestamp_str.replace('Z', '+00:00'))
        except:
            return datetime.now()


class AzureDevOpsDataCollector:
    """Collect communication data from Azure DevOps"""
    
    def __init__(self, personalized_ai: PersonalizedAI):
        self.ai = personalized_ai
        self.azure_client = None
    
    def initialize(self, azure_client):
        """Initialize with Azure DevOps client"""
        self.azure_client = azure_client
        logger.info("Azure DevOps data collector initialized")
    
    def collect_comments_history(self, days: int = 30) -> int:
        """
        Collect work item comments
        
        Args:
            days: Number of days of history to collect
            
        Returns:
            Number of samples collected
        """
        if not self.ai.consent_given:
            logger.warning("Consent not given. Cannot collect data.")
            return 0
        
        logger.info(f"Collecting Azure DevOps comments for last {days} days...")
        samples_collected = 0
        
        try:
            # Get work items updated in the last N days
            cutoff_date = datetime.now() - timedelta(days=days)
            
            # This would query Azure DevOps for work items
            # Placeholder for actual implementation
            work_items = self._get_recent_work_items(days)
            
            for work_item in work_items:
                work_item_id = work_item.get('id')
                
                # Get comments/discussions
                comments = self._get_work_item_comments(work_item_id)
                
                for i in range(1, len(comments)):
                    prev_comment = comments[i-1]
                    current_comment = comments[i]
                    
                    # Check if current comment is from user
                    if self._is_user_comment(current_comment):
                        trigger = self._extract_comment_text(prev_comment)
                        response = self._extract_comment_text(current_comment)
                        
                        self.ai.add_communication_sample(
                            source='azure_devops',
                            context_type='comment',
                            trigger=trigger,
                            response=response,
                            metadata={
                                'work_item_id': work_item_id,
                                'work_item_type': work_item.get('type', 'unknown')
                            }
                        )
                        samples_collected += 1
            
            logger.info(f"Collected {samples_collected} Azure DevOps comment samples")
            return samples_collected
            
        except Exception as e:
            logger.error(f"Error collecting Azure DevOps data: {e}")
            return samples_collected
    
    def _get_recent_work_items(self, days: int) -> List[Dict]:
        """Get recently updated work items from Azure DevOps"""
        if not self.azure_client:
            logger.debug("Azure client not initialized")
            return []

        try:
            import requests
            from requests.auth import HTTPBasicAuth
            from datetime import datetime, timedelta

            # Get Azure DevOps credentials from environment/config
            try:
                from backend.config import azure_org, azure_pat
                org = azure_org()
                pat = azure_pat()
            except (ImportError, TypeError):
                import os
                org = os.getenv('ORGANIZATION') or os.getenv('AZURE_ORG')
                pat = os.getenv('AZURE_DEVOPS_PAT') or os.getenv('AZURE_API_KEY')

            if not org or not pat:
                logger.warning("Azure DevOps credentials not configured")
                return []

            # Get user's projects (simplified - could be parameterized)
            cutoff_date = (datetime.now() - timedelta(days=days)).isoformat()

            # WIQL query for recently updated items
            wiql_query = {
                "query": f"""
                SELECT [System.Id], [System.Title], [System.WorkItemType],
                       [System.State], [System.ChangedDate]
                FROM WorkItems
                WHERE [System.ChangedDate] >= '{cutoff_date}'
                ORDER BY [System.ChangedDate] DESC
                """
            }

            # Query work items across all projects
            # Note: This requires PROJECT to be specified in config
            # For now, we'll attempt to get from the first available project
            base_url = f"https://dev.azure.com/{org}"

            # Try to get projects first
            from backend.config import http_timeout_short
            projects_url = f"{base_url}/_apis/projects?api-version=7.1"
            projects_resp = requests.get(
                projects_url,
                auth=HTTPBasicAuth('', pat),
                timeout=http_timeout_short()
            )

            if projects_resp.status_code != 200:
                logger.warning(f"Could not fetch Azure DevOps projects: {projects_resp.status_code}")
                return []

            projects = projects_resp.json().get('value', [])
            if not projects:
                logger.warning("No Azure DevOps projects found")
                return []

            work_items = []

            # Query the first project for recent work items
            project = projects[0]['name']
            wiql_url = f"{base_url}/{project}/_apis/wit/wiql?api-version=7.1"

            wiql_resp = requests.post(
                wiql_url,
                json=wiql_query,
                auth=HTTPBasicAuth('', pat),
                timeout=http_timeout_short()
            )

            if wiql_resp.status_code == 200:
                items = wiql_resp.json().get('workItems', [])
                # Limit to 50 items to avoid excessive API calls
                for item in items[:50]:
                    work_items.append({
                        'id': item.get('id'),
                        'url': item.get('url'),
                        'type': 'work_item'
                    })

                logger.info(f"Found {len(work_items)} recently updated work items")
            else:
                logger.warning(f"WIQL query failed: {wiql_resp.status_code}")

            return work_items

        except ImportError:
            logger.debug("requests library not available for Azure DevOps API")
            return []
        except Exception as e:
            logger.error(f"Error fetching Azure DevOps work items: {e}")
            return []
    
    def _get_work_item_comments(self, work_item_id: int) -> List[Dict]:
        """Get comments for a work item from Azure DevOps"""
        if not self.azure_client:
            logger.debug("Azure client not initialized")
            return []

        try:
            import requests
            from requests.auth import HTTPBasicAuth

            # Get Azure DevOps credentials
            try:
                from backend.config import azure_org, azure_pat
                org = azure_org()
                pat = azure_pat()
            except (ImportError, TypeError):
                import os
                org = os.getenv('ORGANIZATION') or os.getenv('AZURE_ORG')
                pat = os.getenv('AZURE_DEVOPS_PAT') or os.getenv('AZURE_API_KEY')

            if not org or not pat:
                logger.warning("Azure DevOps credentials not configured")
                return []

            # Get projects to find which one contains this work item
            from backend.config import http_timeout_short
            base_url = f"https://dev.azure.com/{org}"
            projects_url = f"{base_url}/_apis/projects?api-version=7.1"

            projects_resp = requests.get(
                projects_url,
                auth=HTTPBasicAuth('', pat),
                timeout=http_timeout_short()
            )

            if projects_resp.status_code != 200:
                return []

            projects = projects_resp.json().get('value', [])

            comments = []

            # Try each project to find the work item and its comments
            for project in projects:
                project_name = project['name']

                # Get comments for this work item
                comments_url = f"{base_url}/{project_name}/_apis/wit/workItems/{work_item_id}/comments?api-version=7.1"

                comments_resp = requests.get(
                    comments_url,
                    auth=HTTPBasicAuth('', pat),
                    timeout=http_timeout_short()
                )

                if comments_resp.status_code == 200:
                    comments_data = comments_resp.json().get('comments', [])

                    for comment in comments_data:
                        comments.append({
                            'id': comment.get('id'),
                            'text': comment.get('text', ''),
                            'author': {
                                'email': comment.get('createdBy', {}).get('uniqueName', ''),
                                'name': comment.get('createdBy', {}).get('displayName', '')
                            },
                            'timestamp': comment.get('createdDate'),
                            'content': comment.get('content', '')
                        })

                    logger.debug(f"Found {len(comments)} comments for work item {work_item_id}")
                    break  # Found the work item, no need to check other projects

            return comments

        except ImportError:
            logger.debug("requests library not available for Azure DevOps API")
            return []
        except Exception as e:
            logger.error(f"Error fetching Azure DevOps comments: {e}")
            return []
    
    def _is_user_comment(self, comment: Dict) -> bool:
        """Check if comment is from the user"""
        author = comment.get('author', {})
        return author.get('email', '') == self.ai.user_email
    
    def _extract_comment_text(self, comment: Dict) -> str:
        """Extract clean comment text"""
        return comment.get('text', '').strip()


class OutlookDataCollector:
    """Collect communication data from Outlook"""
    
    def __init__(self, personalized_ai: PersonalizedAI):
        self.ai = personalized_ai
        self.graph_client = None
    
    def initialize(self, graph_client):
        """Initialize with Microsoft Graph client"""
        self.graph_client = graph_client
        logger.info("Outlook data collector initialized")
    
    def collect_sent_emails(self, days: int = 30) -> int:
        """
        Collect sent email history
        
        Args:
            days: Number of days of history to collect
            
        Returns:
            Number of samples collected
        """
        if not self.ai.consent_given:
            logger.warning("Consent not given. Cannot collect data.")
            return 0
        
        logger.info(f"Collecting Outlook sent emails for last {days} days...")
        samples_collected = 0
        
        try:
            cutoff_date = datetime.now() - timedelta(days=days)
            
            # Get sent emails
            sent_emails = self._get_sent_emails(days)
            
            for email in sent_emails:
                email_id = email.get('id')
                
                # Check if this is a reply
                if self._is_reply(email):
                    # Get the original message being replied to
                    trigger = self._get_original_message(email)
                    response = self._extract_email_body(email)
                    
                    sent_time = self._parse_timestamp(email.get('sentDateTime'))
                    
                    if sent_time >= cutoff_date:
                        self.ai.add_communication_sample(
                            source='outlook',
                            context_type='email',
                            trigger=trigger,
                            response=response,
                            metadata={
                                'email_id': email_id,
                                'subject': email.get('subject', ''),
                                'timestamp': sent_time.isoformat()
                            }
                        )
                        samples_collected += 1
            
            logger.info(f"Collected {samples_collected} Outlook email samples")
            return samples_collected
            
        except Exception as e:
            logger.error(f"Error collecting Outlook data: {e}")
            return samples_collected
    
    def _get_sent_emails(self, days: int) -> List[Dict]:
        """Get sent emails from Outlook using Microsoft Graph API"""
        if not self.graph_client:
            logger.error("Graph client not initialized")
            return []

        try:
            from datetime import datetime, timedelta

            sent_emails = []
            cutoff_date = datetime.now() - timedelta(days=days)

            # Get messages from Sent folder
            # Note: Microsoft Graph API uses delta queries and filters
            try:
                # Try using graph client's inbox method and filter by sent folder
                if hasattr(self.graph_client, 'get_inbox'):
                    # This assumes graph_client has a method to get sent items
                    # Typical Graph API endpoint: /me/mailFolders/sentItems/messages
                    messages = self.graph_client.get_inbox(folder='sentItems')

                    for message in messages:
                        sent_time_str = message.get('sentDateTime')
                        if sent_time_str:
                            sent_time = self._parse_timestamp(sent_time_str)
                            if sent_time >= cutoff_date:
                                sent_emails.append({
                                    'id': message.get('id'),
                                    'subject': message.get('subject', ''),
                                    'sentDateTime': sent_time_str,
                                    'body': message.get('bodyPreview', ''),
                                    'from': message.get('from', {}),
                                    'toRecipients': message.get('toRecipients', []),
                                    'isRead': message.get('isRead', False)
                                })

                    logger.info(f"Found {len(sent_emails)} sent emails in last {days} days")
                    return sent_emails

            except AttributeError:
                logger.debug("Graph client get_inbox method not available")

            # Fallback: if graph_client has different structure, try direct API call
            if hasattr(self.graph_client, '_client'):
                try:
                    # Direct Microsoft Graph API call using the internal client
                    query_filter = f"sentDateTime ge {cutoff_date.isoformat()}Z"
                    endpoint = f"/me/mailFolders/sentItems/messages?$filter={query_filter}&$top=50"

                    # This is a generic approach - actual implementation depends on graph_client structure
                    logger.debug(f"Using direct Graph API endpoint: {endpoint}")
                    # Would call: response = self.graph_client._client.get(endpoint)
                    # For now, returning empty to avoid errors
                    pass

                except Exception as e:
                    logger.debug(f"Direct Graph API call failed: {e}")

            return sent_emails

        except Exception as e:
            logger.error(f"Error getting sent emails: {e}")
            return []
    
    def _is_reply(self, email: Dict) -> bool:
        """Check if email is a reply"""
        subject = email.get('subject', '')
        return subject.startswith('RE:') or subject.startswith('Re:')
    
    def _get_original_message(self, email: Dict) -> str:
        """Extract the original message being replied to from email body"""
        try:
            body = email.get('body', {})
            content = body.get('content', '')

            if not content:
                return "Original message not available"

            # Remove HTML tags for parsing
            import re
            content_plain = re.sub(r'<[^>]+>', '', content)

            # Look for common quoted text markers
            lines = content_plain.split('\n')
            original_lines = []
            in_original = False

            for line in lines:
                # Check for quote markers
                if line.strip().startswith('>') or \
                   line.strip().startswith('-----Original Message-----') or \
                   'wrote:' in line or \
                   'On' in line and 'wrote' in line:
                    in_original = True

                if in_original:
                    original_lines.append(line)

            if original_lines:
                original_text = '\n'.join(original_lines).strip()
                # Limit to first 500 chars to avoid too much context
                return original_text[:500] if len(original_text) > 500 else original_text

            # If no quoted text found, return first 200 chars as context
            return content_plain[:200] if len(content_plain) > 200 else content_plain

        except Exception as e:
            logger.error(f"Error extracting original message: {e}")
            return "Original message not available"
    
    def _extract_email_body(self, email: Dict) -> str:
        """Extract clean email body (user's response only)"""
        body = email.get('body', {})
        content = body.get('content', '')
        
        # Remove HTML tags
        import re
        content = re.sub(r'<[^>]+>', '', content)
        
        # Remove quoted text (lines starting with >)
        lines = content.split('\n')
        user_lines = [line for line in lines if not line.strip().startswith('>')]
        
        return '\n'.join(user_lines).strip()
    
    def _parse_timestamp(self, timestamp_str: str) -> datetime:
        """Parse ISO timestamp"""
        try:
            return datetime.fromisoformat(timestamp_str.replace('Z', '+00:00'))
        except:
            return datetime.now()


class DataCollectionOrchestrator:
    """Orchestrates data collection from all sources"""
    
    def __init__(self, user_email: str):
        self.ai = PersonalizedAI(user_email)
        self.teams_collector = TeamsDataCollector(self.ai)
        self.azure_collector = AzureDevOpsDataCollector(self.ai)
        self.outlook_collector = OutlookDataCollector(self.ai)
    
    def initialize_collectors(self, graph_client=None, azure_client=None):
        """Initialize all collectors with their respective clients"""
        if graph_client:
            self.teams_collector.initialize(graph_client)
            self.outlook_collector.initialize(graph_client)
        
        if azure_client:
            self.azure_collector.initialize(azure_client)
    
    def collect_all_data(self, days: int = 30) -> Dict[str, int]:
        """
        Collect data from all sources
        
        Args:
            days: Number of days of history to collect
            
        Returns:
            Dictionary with counts per source
        """
        if not self.ai.consent_given:
            print("⚠️  Data collection consent not given.")
            print("   Run: python personalized_ai.py request-consent")
            return {}
        
        print(f"\n📥 Collecting communication data (last {days} days)...")
        print("=" * 70)
        
        results = {}
        
        # Collect from Teams
        print("📱 Collecting from MS Teams...")
        results['teams'] = self.teams_collector.collect_chat_history(days)
        print(f"   ✓ Collected {results['teams']} samples")
        
        # Collect from Azure DevOps
        print("🔷 Collecting from Azure DevOps...")
        results['azure_devops'] = self.azure_collector.collect_comments_history(days)
        print(f"   ✓ Collected {results['azure_devops']} samples")
        
        # Collect from Outlook
        print("📧 Collecting from Outlook...")
        results['outlook'] = self.outlook_collector.collect_sent_emails(days)
        print(f"   ✓ Collected {results['outlook']} samples")
        
        total = sum(results.values())
        print()
        print(f"✅ Total samples collected: {total}")
        print()
        
        # Update profile with new data
        if total > 0:
            print("🧠 Updating AI profile with new data...")
            self.ai._update_profile()
            print("   ✓ Profile updated")
        
        return results
    
    def show_profile_summary(self):
        """Display the current profile summary"""
        print(self.ai.get_profile_summary())


# CLI interface
if __name__ == "__main__":
    import sys
    
    if len(sys.argv) < 2:
        print("Usage: python data_collectors.py <command> [user_email] [days]")
        print("Commands:")
        print("  collect - Collect data from all sources")
        print("  show-profile - Show current profile")
        sys.exit(1)
    
    command = sys.argv[1]
    user_email = sys.argv[2] if len(sys.argv) > 2 else "user@example.com"
    days = int(sys.argv[3]) if len(sys.argv) > 3 else 30
    
    orchestrator = DataCollectionOrchestrator(user_email)
    
    if command == "collect":
        # In real usage, you would pass actual clients here
        # orchestrator.initialize_collectors(graph_client, azure_client)
        orchestrator.collect_all_data(days)
    elif command == "show-profile":
        orchestrator.show_profile_summary()
    else:
        print(f"Unknown command: {command}")
