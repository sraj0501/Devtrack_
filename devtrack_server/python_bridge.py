#!/usr/bin/env python3
"""
DevTrack Python Bridge - IPC Handler

This script receives triggers from the Go daemon via IPC and processes them.
It handles commit triggers and timer triggers, prompting for user input and
sending responses back to Go.
"""

import asyncio
import sys
import os
import time
import logging
from pathlib import Path

# Environment variables must be set before this process starts.
# The Go daemon passes its full environment to this subprocess automatically.
# For standalone local use: source .env into your shell before running.
sys.path.insert(0, os.path.dirname(__file__))

# Set up logging first
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

# Add backend to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'backend'))

from backend.ipc_client import (  # noqa: E402
    IPCClient, IPCMessage, MessageType,
    create_ack_message, create_task_update_message,
    create_response_message
)

# Import NLP parser (will be None if spaCy not installed)
try:
    from backend.nlp_parser import NLPTaskParser, parse_task
    nlp_available = True
except (ImportError, RuntimeError) as e:
    logger.warning(f"NLP parser not available: {e}")
    logger.warning("Install spaCy and model: pip install spacy && python -m spacy download en_core_web_sm")
    nlp_available = False
    NLPTaskParser = None
    parse_task = None

# Import Git Diff Analyzer
try:
    from backend.git_diff_analyzer import GitDiffAnalyzer
    git_analyzer_available = True
except ImportError as e:
    logger.warning(f"Git diff analyzer not available: {e}")
    git_analyzer_available = False
    GitDiffAnalyzer = None

# Import Description Enhancer (Ollama-based)
try:
    from backend.description_enhancer import DescriptionEnhancer, EnhancedDescription
    description_enhancer_available = True
except ImportError as e:
    logger.warning(f"Description enhancer not available: {e}")
    description_enhancer_available = False
    DescriptionEnhancer = None
    EnhancedDescription = None

# Import TUI for user prompts
try:
    from backend.user_prompt import DevTrackTUI, UserResponse
    tui_available = True
except ImportError as e:
    logger.warning(f"TUI not available: {e}")
    tui_available = False
    DevTrackTUI = None
    UserResponse = None

# Import Daily Report Generator (Phase 5)
try:
    from backend.daily_report_generator import (
        DailyReportGenerator, EnhancedReport, ReportStyle, OutputFormat, WeeklyReport
    )
    daily_report_available = True
except ImportError as e:
    logger.warning(f"Daily report generator not available: {e}")
    daily_report_available = False
    DailyReportGenerator = None
    EnhancedReport = None
    ReportStyle = None
    OutputFormat = None
    WeeklyReport = None

# Import backend config for Azure sync settings and other config access
try:
    import backend.config as config
except ImportError:
    config = None

# Import Task Repository and Jira client (Phase 4)
try:
    from backend.task_matcher import TaskRepository, TaskMatcher
    from backend.config import jira_url, jira_api_token
    task_repo_available = True
except ImportError as e:
    logger.warning(f"Task repository not available: {e}")
    task_repo_available = False
    TaskRepository = None
    TaskMatcher = None

# Import git-sage conflict resolver and work update enhancer (Phase 3)
try:
    from backend.conflict_auto_resolver import auto_resolve_merge_conflicts
    conflict_resolver_available = True
except ImportError as e:
    logger.debug(f"Conflict resolver not available: {e}")
    conflict_resolver_available = False

try:
    from backend.work_update_enhancer import enhance_work_update_prompt
    work_enhancer_available = True
except ImportError as e:
    logger.debug(f"Work update enhancer not available: {e}")
    work_enhancer_available = False

# Import Personalized AI (Phase 6 - Talk Like You)
try:
    from backend.personalized_ai import PersonalizedAI
    personalized_ai_available = True
except ImportError as e:
    logger.debug(f"Personalized AI not available: {e}")
    personalized_ai_available = False
    PersonalizedAI = None

# Import Azure DevOps client for bidirectional sync
try:
    from backend.azure.client import AzureDevOpsClient
    azure_client_available = True
except ImportError as e:
    logger.debug(f"Azure DevOps client not available: {e}")
    azure_client_available = False
    AzureDevOpsClient = None

# Import GitLab client for bidirectional sync
try:
    from backend.gitlab.client import GitLabClient
    gitlab_client_available = True
except ImportError as e:
    logger.debug(f"GitLab client not available: {e}")
    gitlab_client_available = False
    GitLabClient = None

# Import GitHub client for bidirectional sync
try:
    from backend.github.client import GitHubClient
    github_client_available = True
except ImportError as e:
    logger.debug(f"GitHub client not available: {e}")
    github_client_available = False
    GitHubClient = None

# Import WorkspaceRouter for per-repo PM platform routing
try:
    from backend.workspace_router import WorkspaceRouter
    workspace_router_available = True
except ImportError as e:
    logger.debug(f"WorkspaceRouter not available: {e}")
    workspace_router_available = False
    WorkspaceRouter = None

# Import runtime-narrative for structured observability (graceful degradation)
try:
    from runtime_narrative import story as _story, stage as _stage
    _narrative_available = True
    logger.debug("runtime-narrative available: structured failure reporting enabled")
except ImportError:
    from contextlib import contextmanager

    @contextmanager
    def _story(name, **kwargs):
        yield

    @contextmanager
    def _stage(name):
        yield

    _narrative_available = False


class DevTrackBridge:
    """Main bridge between Go daemon and Python AI layer"""
    
    def __init__(self):
        self.ipc_client = IPCClient()
        self.running = False
        self.trigger_count = {"commit": 0, "timer": 0}
        
        # Initialize NLP parser if available
        self.nlp_parser = None
        if nlp_available:
            try:
                self.nlp_parser = NLPTaskParser(use_ollama=True)
                logger.info("✓ NLP parser initialized")
            except Exception as e:
                logger.warning(f"Could not initialize NLP parser: {e}")
        
        # Initialize Git Diff Analyzer if available
        self.git_analyzer = None
        if git_analyzer_available:
            try:
                self.git_analyzer = GitDiffAnalyzer()
                logger.info("✓ Git diff analyzer initialized")
            except Exception as e:
                logger.warning(f"Could not initialize git analyzer: {e}")
        
        # Initialize Description Enhancer (Ollama) if available
        self.description_enhancer = None
        if description_enhancer_available:
            try:
                self.description_enhancer = DescriptionEnhancer()
                if self.description_enhancer.is_available():
                    logger.info("✓ Description enhancer initialized (Ollama available)")
                else:
                    logger.info("✓ Description enhancer initialized (Ollama not available, using fallback)")
            except Exception as e:
                logger.warning(f"Could not initialize description enhancer: {e}")
        
        # Initialize TUI if available
        self.tui = None
        if tui_available:
            try:
                self.tui = DevTrackTUI()
                logger.info("✓ TUI initialized")
            except Exception as e:
                logger.warning(f"Could not initialize TUI: {e}")
        
        # Initialize Daily Report Generator (Phase 5) if available
        self.report_generator = None
        if daily_report_available:
            try:
                self.report_generator = DailyReportGenerator()
                if self.report_generator.check_ollama_available():
                    logger.info("✓ Daily report generator initialized (AI-enhanced)")
                else:
                    logger.info("✓ Daily report generator initialized (basic mode)")
            except Exception as e:
                logger.warning(f"Could not initialize daily report generator: {e}")

        # Initialize Task Repository with Jira (Phase 4)
        self.task_repo = None
        if task_repo_available:
            try:
                self.task_repo = TaskRepository()
                if jira_url() and jira_api_token():
                    from backend.jira import JiraClient
                    self.task_repo.initialize_jira(JiraClient())
                    logger.info("✓ Jira integration initialized")
                else:
                    logger.info("✓ Task repository initialized (Jira not configured)")
            except Exception as e:
                logger.warning(f"Could not initialize task repository: {e}")

        # Initialize Azure DevOps client for bidirectional sync
        self.azure_client = None
        if azure_client_available and task_repo_available:
            try:
                client = AzureDevOpsClient()
                if client.is_configured():
                    self.azure_client = client
                    if self.task_repo:
                        self.task_repo.initialize_azure(client)
                    logger.info("✓ Azure DevOps integration initialized")
                else:
                    logger.info("✓ Azure DevOps client available (not configured)")
            except Exception as e:
                logger.warning(f"Could not initialize Azure DevOps client: {e}")

        # Initialize TaskMatcher for fuzzy/semantic work item matching
        self.task_matcher = None
        if task_repo_available and TaskMatcher is not None:
            try:
                self.task_matcher = TaskMatcher(use_semantic=False)
                logger.info("✓ Task matcher initialized")
            except Exception as e:
                logger.warning(f"Could not initialize task matcher: {e}")

        # Initialize GitLab client for bidirectional sync
        self.gitlab_client = None
        if gitlab_client_available:
            try:
                client = GitLabClient()
                if client.is_configured():
                    self.gitlab_client = client
                    logger.info("✓ GitLab integration initialized")
                else:
                    logger.info("✓ GitLab client available (not configured)")
            except Exception as e:
                logger.warning(f"Could not initialize GitLab client: {e}")

        # Initialize GitHub client for bidirectional sync
        self.github_client = None
        if github_client_available:
            try:
                client = GitHubClient()
                if client.is_configured():
                    self.github_client = client
                    logger.info("✓ GitHub integration initialized")
                else:
                    logger.info("✓ GitHub client available (not configured)")
            except Exception as e:
                logger.warning(f"Could not initialize GitHub client: {e}")

        # Initialize WorkspaceRouter for workspace-aware PM routing
        self.workspace_router = None
        if workspace_router_available and WorkspaceRouter is not None:
            try:
                self.workspace_router = WorkspaceRouter(
                    azure_client=self.azure_client,
                    gitlab_client=self.gitlab_client,
                    github_client=self.github_client,
                )
                logger.info("✓ WorkspaceRouter initialized")
            except Exception as e:
                logger.warning(f"Could not initialize WorkspaceRouter: {e}")

        # Initialize Personalized AI (Phase 6 - Talk Like You)
        self.personalized_ai = None
        if personalized_ai_available:
            try:
                # Try to get user email from environment or config
                try:
                    from backend.config import user_email as get_user_email
                    user_email = get_user_email()
                except (ImportError, AttributeError, TypeError):
                    # Fallback to environment variable or placeholder
                    user_email = os.getenv('USER_EMAIL', os.getenv('USER', 'user@example.com'))

                self.personalized_ai = PersonalizedAI(user_email)
                if self.personalized_ai.consent_given:
                    logger.info("✓ Personalized AI initialized (consent given)")
                    if self.personalized_ai.profile:
                        logger.info(f"  User profile: {self.personalized_ai.profile.total_samples} samples")
                else:
                    logger.info("✓ Personalized AI available (consent not yet given)")
            except Exception as e:
                logger.warning(f"Could not initialize personalized AI: {e}")

    def handle_commit_trigger(self, msg: IPCMessage):
        """Handle commit trigger from Go daemon"""
        self.trigger_count["commit"] += 1
        
        logger.info("=" * 60)
        logger.info(f"🎯 COMMIT TRIGGER #{self.trigger_count['commit']}")
        logger.info("=" * 60)
        
        data = msg.data
        logger.info(f"Repository: {data.get('repo_path', 'unknown')}")
        logger.info(f"Commit Hash: {data.get('commit_hash', 'unknown')[:12]}")
        logger.info(f"Author: {data.get('author', 'unknown')}")
        logger.info(f"Message: {data.get('commit_message', '')}")
        
        files = data.get('files_changed', [])
        if files:
            logger.info(f"Files Changed: {len(files)}")
            for f in files[:5]:  # Show first 5 files
                logger.info(f"  - {f}")
            if len(files) > 5:
                logger.info(f"  ... and {len(files) - 5} more")
        
        logger.info("")
        
        # Send acknowledgment
        ack = create_ack_message(msg.id)
        self.ipc_client.send_message(ack)

        # Auto-link commit to active work session (non-fatal)
        commit_hash = data.get('commit_hash', '')
        if commit_hash:
            try:
                from backend.work_tracker.session_store import WorkSessionStore
                _store = WorkSessionStore()
                _active = _store.get_active_session()
                if _active:
                    _store.append_commit(_active['id'], commit_hash)
                    logger.info(f"📎 Commit {commit_hash[:12]} linked to work session #{_active['id']}")
            except Exception as _e:
                logger.debug(f"Work session commit link failed (non-fatal): {_e}")

        # Parse commit message with NLP if available
        commit_msg = data.get('commit_message', '')
        repo_path = data.get('repo_path', '')
        commit_hash = data.get('commit_hash', '')
        
        # Check if we should use git diff analysis (no project management system)
        use_diff_analysis = False
        if self.git_analyzer and self.git_analyzer.is_project_management_connected():
            logger.info("ℹ️  Project management system connected, using standard parsing")
        elif self.git_analyzer:
            logger.info("ℹ️  No project management system detected, will analyze git diff")
            use_diff_analysis = True
        
        # If using diff analysis, get AI summary
        ai_summary = None
        if use_diff_analysis and repo_path and commit_hash:
            try:
                logger.info("🔍 Analyzing commit changes with AI...")
                analysis = self.git_analyzer.process_commit(
                    repo_path=repo_path,
                    commit_hash=commit_hash,
                    commit_message=commit_msg,
                    files_changed=files or []
                )
                
                logger.info(f"   Type: {analysis.get('type', 'unknown')}")
                logger.info(f"   Impact: {analysis.get('impact', 'unknown')}")
                logger.info(f"   Summary: {analysis.get('summary', '')[:80]}...")
                
                ai_summary = analysis
                
                # Use AI-generated summary as description
                commit_msg = analysis.get('summary', commit_msg)
                
            except Exception as e:
                logger.error(f"Error analyzing diff: {e}")
                use_diff_analysis = False
        
        if self.nlp_parser and commit_msg:
            logger.info("📝 Parsing commit message with NLP...")
            try:
                parsed = self.nlp_parser.parse(commit_msg, repo_path=repo_path)
                logger.info(f"   Project: {parsed.project}")
                logger.info(f"   Ticket:  {parsed.ticket_id}")
                logger.info(f"   Action:  {parsed.action_verb}")
                logger.info(f"   Status:  {parsed.status}")
                logger.info(f"   Confidence: {parsed.confidence:.2f}")

                # Log git context if available (Phase 1 enhancement)
                if hasattr(parsed, 'git_context') and parsed.git_context and parsed.git_context.get('branch'):
                    git_branch = parsed.git_context['branch'].get('branch', '')
                    pr_num = parsed.git_context['branch'].get('issue_number', '')
                    if git_branch:
                        logger.info(f"   Git Context: {git_branch}" + (f" (PR #{pr_num})" if pr_num else ""))
                
                # Build description
                description = parsed.description or commit_msg
                if ai_summary and use_diff_analysis:
                    # Enhance with AI analysis
                    description = f"[{ai_summary.get('type', 'update')}] {description}"

                # Route to the correct PM platform
                azure_work_item_id = None
                synced_platform = None
                commit_info_dict = {
                    "commit_hash": data.get("commit_hash", ""),
                    "commit_message": data.get("commit_message", ""),
                    "author": data.get("author", ""),
                }
                workspace_settings = {
                    "assignee": data.get("pm_assignee", ""),
                    "iteration_path": data.get("pm_iteration_path", ""),
                    "area_path": data.get("pm_area_path", ""),
                    "milestone": data.get("pm_milestone", 0),
                }
                azure_work_item_id, synced_platform = self._route_pm_sync(
                    pm_platform=data.get("pm_platform", ""),
                    pm_project=data.get("pm_project", ""),
                    description=description,
                    ticket_id=parsed.ticket_id or "",
                    status=parsed.status or "in_progress",
                    commit_info=commit_info_dict,
                    workspace_settings=workspace_settings,
                )

                # Send task update with parsed data
                task_update = create_task_update_message(
                    project=parsed.project or "automation_tools",
                    ticket_id=parsed.ticket_id or "",
                    description=description,
                    status=parsed.status or "in_progress",
                    time_spent=parsed.time_spent or "",
                    synced=synced_platform is not None,
                    azure_work_item_id=azure_work_item_id,
                    synced_platform=synced_platform,
                )
                self.ipc_client.send_message(task_update)
                logger.info("✓ Sent parsed task update to Go daemon")
            except Exception as e:
                logger.error(f"Error parsing commit message: {e}")
        else:
            # Fallback: send basic task update (with AI summary if available)
            if use_diff_analysis and ai_summary:
                logger.info("ℹ️  NLP parser not available, using AI-generated summary")
                description = f"[{ai_summary.get('type', 'update')}] {ai_summary.get('summary', commit_msg)}"
            else:
                logger.info("ℹ️  NLP parser not available, sending basic update")
                description = commit_msg

            # Route to the correct PM platform (fallback path — no NLP)
            azure_work_item_id = None
            synced_platform = None
            commit_info_dict = {
                "commit_hash": data.get("commit_hash", ""),
                "commit_message": data.get("commit_message", ""),
                "author": data.get("author", ""),
            }
            workspace_settings = {
                "assignee": data.get("pm_assignee", ""),
                "iteration_path": data.get("pm_iteration_path", ""),
                "area_path": data.get("pm_area_path", ""),
                "milestone": data.get("pm_milestone", 0),
            }
            azure_work_item_id, synced_platform = self._route_pm_sync(
                pm_platform=data.get("pm_platform", ""),
                pm_project=data.get("pm_project", ""),
                description=description,
                ticket_id="",
                status="in_progress",
                commit_info=commit_info_dict,
                workspace_settings=workspace_settings,
            )

            task_update = create_task_update_message(
                project="automation_tools",
                ticket_id="",
                description=description,
                status="in_progress",
                time_spent="",
                synced=synced_platform is not None,
                azure_work_item_id=azure_work_item_id,
                synced_platform=synced_platform,
            )
            self.ipc_client.send_message(task_update)
        
        logger.info("")
        logger.info("📝 Commit processing complete")
        if use_diff_analysis and ai_summary:
            logger.info(f"   ✓ AI-enhanced summary generated")
        if self.nlp_parser:
            logger.info(f"   ✓ NLP parsing completed")
        if self.azure_client and config and config.is_azure_sync_enabled():
            logger.info(f"   ✓ Azure DevOps sync attempted")
        logger.info("")
    
    def handle_timer_trigger(self, msg: IPCMessage):
        """Handle timer trigger from Go daemon - prompts user for work update"""
        self.trigger_count["timer"] += 1
        
        logger.info("=" * 60)
        logger.info(f"⏰ TIMER TRIGGER #{self.trigger_count['timer']}")
        logger.info("=" * 60)
        
        data = msg.data
        interval_mins = data.get('interval_mins', 0)
        logger.info(f"Timestamp: {data.get('timestamp', '')}")
        logger.info(f"Interval: {interval_mins} minutes")
        logger.info(f"Trigger Count: {data.get('trigger_count', 0)}")
        logger.info("")
        
        # Send acknowledgment
        ack = create_ack_message(msg.id)
        self.ipc_client.send_message(ack)
        
        # Phase 3 Implementation: TUI + NLP + Ollama Enhancement

        # Check for an active work session — pre-fill ticket and skip time prompt
        _active_session = None
        _session_ticket_ref = ""
        try:
            from backend.work_tracker.session_store import WorkSessionStore
            _session_store = WorkSessionStore()
            _active_session = _session_store.get_active_session()
            if _active_session:
                _session_ticket_ref = _active_session.get("ticket_ref") or ""
                logger.info(f"🟢 Active work session #{_active_session['id']}"
                            + (f" (ticket: {_session_ticket_ref})" if _session_ticket_ref else ""))
        except Exception as _e:
            logger.debug(f"Work session check failed (non-fatal): {_e}")

        # Step 1: Prompt user for work update via TUI
        user_input = None
        if self.tui:
            try:
                context = {
                    "interval_mins": interval_mins,
                    "trigger_count": data.get('trigger_count', 0),
                    # When a session is active, pre-fill ticket and hide time prompt
                    "active_session": _active_session is not None,
                    "session_ticket_ref": _session_ticket_ref,
                }
                response = self.tui.prompt_work_update(context)
                
                if response.cancelled:
                    logger.info("⚠️  User cancelled the prompt")
                    self.tui.print_warning("Update skipped - you can update later")
                    return
                elif response.timed_out:
                    logger.info("⚠️  Prompt timed out")
                    self.tui.print_warning("Prompt timed out - will ask again next interval")
                    return
                elif not response.is_valid():
                    logger.info("ℹ️  Empty response - no update to process")
                    self.tui.print_info("No input received - skipping update")
                    return
                
                user_input = response.raw_input
                logger.info(f"📝 User input received: {user_input[:50]}...")
                
            except Exception as e:
                logger.error(f"Error prompting user: {e}")
                return
        else:
            logger.warning("TUI not available - cannot prompt user interactively")
            logger.info("📝 Next steps (TUI not available):")
            logger.info("   - Install TUI dependencies or use non-interactive mode")
            return
        
        # Step 2: Enhance work update with git context (Phase 3)
        enhanced_input = user_input
        repo_path = "."
        git_context = None

        if work_enhancer_available and user_input:
            try:
                self.tui.show_progress("Enhancing with git context")
                enhanced_input = enhance_work_update_prompt(user_input, repo_path=repo_path)

                if enhanced_input != user_input:
                    logger.info("✓ Work update enriched with git context")
                    logger.debug(f"Enhanced input:\n{enhanced_input}")

                self.tui.show_progress("Enhancing with git context", done=True)
            except Exception as e:
                logger.debug(f"Could not enhance work update: {e}")
                # Continue with original input

        # Step 3: Parse user input with spaCy NLP
        parsed = None
        if self.nlp_parser and enhanced_input:
            try:
                self.tui.show_progress("Parsing with NLP")
                parsed = self.nlp_parser.parse(enhanced_input, repo_path=repo_path)
                self.tui.show_progress("Parsing with NLP", done=True)

                # Extract git context from parsed task
                git_context = parsed.git_context if hasattr(parsed, 'git_context') else None
                
                # Display parsed information
                self.tui.display_parsed_task(parsed.to_dict())
                
                logger.info("📋 NLP Parsing Results:")
                logger.info(f"   Project: {parsed.project}")
                logger.info(f"   Ticket:  {parsed.ticket_id}")
                logger.info(f"   Action:  {parsed.action_verb}")
                logger.info(f"   Status:  {parsed.status}")
                logger.info(f"   Time:    {parsed.time_spent}")
                logger.info(f"   Confidence: {parsed.confidence:.2f}")
                
            except Exception as e:
                logger.error(f"Error parsing with NLP: {e}")
                # Continue with basic processing

        # Step 4: Enhance description with Ollama
        enhanced = None
        if self.description_enhancer and enhanced_input:
            try:
                self.tui.show_progress("Enhancing description with AI")

                # Build context for enhancement
                enhance_context = {}
                if parsed:
                    if parsed.project:
                        enhance_context["project"] = parsed.project
                    if parsed.ticket_id:
                        enhance_context["ticket_id"] = parsed.ticket_id
                    # Add git context to enhancement context
                    if git_context:
                        enhance_context["branch"] = git_context.get("branch", {}).get("branch")
                        enhance_context["pr_number"] = git_context.get("branch", {}).get("issue_number")

                enhanced = self.description_enhancer.enhance(enhanced_input, enhance_context)
                self.tui.show_progress("Enhancing description with AI", done=True)
                
                # Display enhanced description
                self.tui.display_enhanced_description(enhanced.to_dict())
                
                logger.info("🧠 AI Enhancement Results:")
                logger.info(f"   Category: {enhanced.category}")
                logger.info(f"   Summary: {enhanced.summary}")
                logger.info(f"   Keywords: {', '.join(enhanced.keywords)}")
                
            except Exception as e:
                logger.error(f"Error enhancing description: {e}")
                # Continue with parsed description

        # Step 4.5: Generate personalized response suggestion (Phase 6 - Talk Like You)
        response_suggestion = None
        if self.personalized_ai and self.personalized_ai.consent_given and self.personalized_ai.profile:
            try:
                self.tui.show_progress("Generating personalized response suggestion")

                # Use the work update as context for response generation
                context_type = "chat"  # Default context
                if parsed and parsed.ticket_id:
                    context_type = "comment"  # If it's a ticket update, it's a comment

                response_suggestion = self.personalized_ai.generate_response_suggestion(
                    context_type=context_type,
                    trigger=final_description,
                    additional_context=f"Project: {parsed.project if parsed else 'unknown'}"
                )

                self.tui.show_progress("Generating personalized response suggestion", done=True)

                if response_suggestion and not response_suggestion.startswith("Error"):
                    logger.info("✓ Personalized response suggestion generated")
                    logger.debug(f"Suggestion: {response_suggestion[:100]}...")
                else:
                    logger.debug(f"Could not generate response suggestion: {response_suggestion}")
                    response_suggestion = None

            except Exception as e:
                logger.debug(f"Could not generate response suggestion: {e}")
                # Continue without suggestion - this is optional

        # Step 5: Confirm with user
        final_description = enhanced_input
        if enhanced and enhanced.enhanced:
            final_description = enhanced.enhanced
        elif parsed and parsed.description:
            final_description = parsed.description
        
        # Ask user to confirm
        if self.tui:
            print(f"\n{self.tui.ICON_TASK} Final description:")
            print(f"   {final_description[:100]}...")

            # Show personalized response suggestion if available (Phase 6)
            if response_suggestion:
                print(f"\n💡 Personalized Response Suggestion (Talk Like You):")
                print(f"   {response_suggestion[:150]}...")
                if len(response_suggestion) > 150:
                    print("   [Full suggestion available]")

            confirmed = self.tui.prompt_yes_no("Save this update?", default=True)

            if not confirmed:
                self.tui.print_warning("Update cancelled by user")
                return

        # Step 6: Create and send task update
        project = "automation_tools"
        ticket_id = ""
        status = "in_progress"
        time_spent = ""

        if parsed:
            project = parsed.project or project
            ticket_id = parsed.ticket_id or ticket_id
            status = parsed.status or status
            time_spent = parsed.time_spent or time_spent

        # Pre-fill ticket from active work session when NLP didn't extract one
        if not ticket_id and _session_ticket_ref:
            ticket_id = _session_ticket_ref
            logger.info(f"📎 Ticket pre-filled from active session: {ticket_id}")

        # Add category prefix if enhanced
        if enhanced and enhanced.category and enhanced.category != "other":
            final_description = f"[{enhanced.category}] {final_description}"

        # Step 6.5: Route to the correct PM platform
        azure_work_item_id = None
        synced_platform = None
        workspace_settings = {
            "assignee": data.get("pm_assignee", ""),
            "iteration_path": data.get("pm_iteration_path", ""),
            "area_path": data.get("pm_area_path", ""),
            "milestone": data.get("pm_milestone", 0),
        }
        azure_work_item_id, synced_platform = self._route_pm_sync(
            pm_platform=data.get("pm_platform", ""),
            pm_project=data.get("pm_project", ""),
            description=final_description,
            ticket_id=ticket_id,
            status=status,
            workspace_settings=workspace_settings,
        )

        task_update = create_task_update_message(
            project=project,
            ticket_id=ticket_id,
            description=final_description,
            status=status,
            time_spent=time_spent,
            synced=synced_platform is not None,
            azure_work_item_id=azure_work_item_id,
            synced_platform=synced_platform,
        )
        self.ipc_client.send_message(task_update)
        
        # Success feedback
        if self.tui:
            self.tui.print_success("Work update saved!")
            summary_items = {}
            if project:
                summary_items["Project"] = project
            if ticket_id:
                summary_items["Ticket"] = ticket_id
            if status:
                summary_items["Status"] = status
            if time_spent:
                summary_items["Time"] = time_spent
            if summary_items:
                self.tui.print_section("Update Summary", summary_items, icon="✓")
        
        logger.info("")
        logger.info("✓ Timer trigger processing complete")
        logger.info(f"   ✓ User prompted via TUI")
        if parsed:
            logger.info(f"   ✓ Parsed with spaCy NLP (confidence: {parsed.confidence:.2f})")
        if git_context:
            logger.info(f"   ✓ Git context enriched (branch: {git_context.get('branch', {}).get('branch')})")
        if enhanced:
            logger.info(f"   ✓ Enhanced with Ollama (category: {enhanced.category})")
        if response_suggestion:
            logger.info(f"   ✓ Personalized response suggestion generated")
        if synced_platform:
            logger.info(f"   ✓ Synced to {synced_platform} (work item #{azure_work_item_id})")
        logger.info(f"   ✓ Task update sent to daemon")
        logger.info("")

        # Step 7: Check for and resolve merge conflicts (Phase 3 - git-sage)
        self._check_and_resolve_conflicts()
        
        # Phase 5 - Generate daily report if end of day
        self._check_end_of_day_report()
    
    def _check_end_of_day_report(self):
        """Check if it's end of day and offer to generate report."""
        if not self.report_generator:
            logger.debug("Report generator not available, skipping end-of-day check")
            return
        
        # Check if within end-of-day window (default 6:00 PM ± 15 minutes)
        if not self.report_generator.is_end_of_day():
            return
        
        from datetime import datetime
        is_friday = datetime.now().weekday() == 4  # Friday
        
        logger.info("")
        logger.info("📊 End of day detected - offering report generation")
        
        # Prompt user via TUI if available
        if self.tui:
            # On Fridays, offer weekly report option
            if is_friday:
                logger.info("📅 It's Friday - weekly report option available")
                
                # Ask which report they want
                options = [
                    "Daily report only",
                    "Weekly report (includes all days)",
                    "Skip for now"
                ]
                choice_idx, choice_text = self.tui.prompt_choice(
                    "End of week! Which report would you like?",
                    options=options,
                    default=1  # Weekly is default on Friday
                )
                
                if choice_idx == 2:  # Skip
                    logger.info("User skipped report generation")
                    return
                elif choice_idx == 1:  # Weekly
                    self._generate_weekly_report()
                    return
                # else choice_idx == 0: continue with daily
            else:
                generate = self.tui.prompt_yes_no(
                    "End of day! Would you like to generate your daily report?",
                    default=True
                )
                
                if not generate:
                    logger.info("User declined daily report generation")
                    return
        
        # Generate the daily report
        self._generate_daily_report()
    
    def _generate_daily_report(self, save: bool = True):
        """
        Generate the daily report with AI enhancement.
        
        Args:
            save: Whether to save the report to a file
        """
        if not self.report_generator:
            logger.error("Report generator not available")
            return
        
        logger.info("📊 Generating AI-enhanced daily report...")
        
        # Show progress if TUI available
        if self.tui:
            self.tui.show_progress("Collecting activities...")
        
        # Generate enhanced report
        report = self.report_generator.generate_report(
            style=ReportStyle.PROFESSIONAL if ReportStyle else "professional",
            include_ai=True
        )
        
        if self.tui:
            self.tui.show_progress("Analyzing with AI...")
        
        # Display preview
        if self.tui:
            preview = self.report_generator.get_report_preview(report)
            print("\n" + preview + "\n")
            self.tui.show_progress("Report ready!", done=True)
        
        # Ask about output options
        output_choice = "terminal"
        if self.tui:
            # Simple approach - show to terminal and offer to save
            terminal_report = self.report_generator.format_report(
                report,
                OutputFormat.TERMINAL if OutputFormat else "terminal"
            )
            print("\n" + terminal_report)
            
            if save:
                save_it = self.tui.prompt_yes_no(
                    "Save this report to a file?",
                    default=True
                )
                
                if save_it:
                    saved_path = self.report_generator.save_report(
                        report,
                        OutputFormat.MARKDOWN if OutputFormat else "markdown"
                    )
                    self.tui.print_success(f"Report saved to: {saved_path}")
                    
                    # Also save HTML version
                    html_path = self.report_generator.save_report(
                        report,
                        OutputFormat.HTML if OutputFormat else "html"
                    )
                    logger.info(f"HTML report saved to: {html_path}")
        else:
            # No TUI - just log the report
            terminal_report = self.report_generator.format_report(
                report,
                OutputFormat.TERMINAL if OutputFormat else "terminal"
            )
            logger.info("\n" + terminal_report)
            
            if save:
                saved_path = self.report_generator.save_report(
                    report,
                    OutputFormat.TEXT if OutputFormat else "text"
                )
                logger.info(f"Report saved to: {saved_path}")
        
        # Log summary
        logger.info("")
        logger.info("✓ Daily report generation complete")
        if report.is_ai_enhanced:
            logger.info("   ✓ AI insights generated")
        logger.info(f"   ✓ Activities: {len(report.base_report.activities)}")
        logger.info(f"   ✓ Total hours: {report.base_report.total_hours:.1f}h")
    
    def _generate_weekly_report(self, save: bool = True):
        """
        Generate the weekly report with AI enhancement.
        
        Args:
            save: Whether to save the report to a file
        """
        if not self.report_generator:
            logger.error("Report generator not available")
            return
        
        logger.info("📊 Generating AI-enhanced weekly report...")
        
        # Show progress if TUI available
        if self.tui:
            self.tui.show_progress("Collecting week's activities...")
        
        # Generate weekly report
        weekly_report = self.report_generator.generate_weekly_report(
            include_ai=True
        )
        
        if self.tui:
            self.tui.show_progress("Analyzing week with AI...")
        
        # Display the report
        if self.tui:
            terminal_report = self.report_generator.format_weekly_report(
                weekly_report,
                OutputFormat.TERMINAL if OutputFormat else "terminal"
            )
            print("\n" + terminal_report)
            self.tui.show_progress("Weekly report ready!", done=True)
            
            if save:
                save_it = self.tui.prompt_yes_no(
                    "Save this weekly report to a file?",
                    default=True
                )
                
                if save_it:
                    # Save as markdown
                    import os
                    try:
                        from backend.config import reports_dir as get_reports_dir
                        reports_dir = str(get_reports_dir())
                    except ImportError:
                        try:
                            from backend.utils.paths import fallback_path
                            reports_dir = fallback_path("Data", "reports")
                        except ImportError:
                            from pathlib import Path
                            cur = Path(__file__).resolve().parent
                            reports_dir = str(cur / "Data" / "reports")
                    os.makedirs(reports_dir, exist_ok=True)
                    
                    week_str = weekly_report.week_start.strftime('%Y-%m-%d')
                    ai_suffix = "_ai" if weekly_report.is_ai_enhanced else ""
                    md_path = os.path.join(reports_dir, f"weekly-{week_str}{ai_suffix}.md")
                    
                    content = self.report_generator.format_weekly_report(
                        weekly_report,
                        OutputFormat.MARKDOWN if OutputFormat else "markdown"
                    )
                    with open(md_path, 'w') as f:
                        f.write(content)
                    
                    self.tui.print_success(f"Weekly report saved to: {md_path}")
                    
                    # Also save HTML version
                    html_path = os.path.join(reports_dir, f"weekly-{week_str}{ai_suffix}.html")
                    html_content = self.report_generator.format_weekly_report(
                        weekly_report,
                        OutputFormat.HTML if OutputFormat else "html"
                    )
                    with open(html_path, 'w') as f:
                        f.write(html_content)
                    
                    logger.info(f"HTML weekly report saved to: {html_path}")
        else:
            # No TUI - just log the report
            terminal_report = self.report_generator.format_weekly_report(
                weekly_report,
                OutputFormat.TERMINAL if OutputFormat else "terminal"
            )
            logger.info("\n" + terminal_report)
        
        # Log summary
        logger.info("")
        logger.info("✓ Weekly report generation complete")
        if weekly_report.is_ai_enhanced:
            logger.info("   ✓ AI insights generated")
        logger.info(f"   ✓ Days covered: {len(weekly_report.daily_reports)}")
        logger.info(f"   ✓ Total hours: {weekly_report.total_hours:.1f}h")
        logger.info(f"   ✓ Tasks completed: {weekly_report.total_completed}")

    def _check_and_resolve_conflicts(self):
        """
        Check for merge/rebase conflicts and attempt automatic resolution (Phase 3).
        This runs after work updates to catch any pending conflicts.
        """
        if not conflict_resolver_available:
            logger.debug("Conflict resolver not available, skipping conflict check")
            return

        try:
            logger.debug("Checking for merge/rebase conflicts...")

            # Attempt auto-resolution
            result = auto_resolve_merge_conflicts(repo_path=".")

            if result["status"] == "success":
                # All conflicts resolved
                logger.info("")
                logger.info("🎯 Merge Conflict Resolution (Phase 3)")
                logger.info(f"✓ {result['summary']}")
                logger.info(f"  Auto-resolved {len(result['resolved'])} files:")
                for f in result['resolved']:
                    logger.info(f"    ✓ {f}")

                # Log resolution event
                if self.tui:
                    self.tui.print_success("Merge conflicts auto-resolved!")

            elif result["status"] == "partial":
                # Some resolved, some need manual work
                logger.info("")
                logger.info("🎯 Merge Conflict Resolution (Phase 3)")
                logger.info(f"⚠ {result['summary']}")

                if result['resolved']:
                    logger.info(f"Auto-resolved {len(result['resolved'])} files:")
                    for f in result['resolved']:
                        logger.info(f"  ✓ {f}")

                if result['unresolvable']:
                    logger.info(f"Files needing manual resolution ({len(result['unresolvable'])}):")
                    for f in result['unresolvable']:
                        logger.info(f"  ✗ {f}")

                # Offer to show conflict details
                if self.tui:
                    self.tui.print_warning("Some conflicts need manual resolution")
                    show_details = self.tui.prompt_yes_no(
                        "Show conflict details?",
                        default=False
                    )

                    if show_details:
                        from backend.conflict_auto_resolver import get_conflict_report
                        report = get_conflict_report(repo_path=".")
                        print("\n" + report + "\n")

            elif result["status"] == "failed":
                # No conflicts detected or all need manual work
                if result.get("error"):
                    logger.debug(f"Could not auto-resolve conflicts: {result['error']}")
                else:
                    logger.debug("No conflicts detected")

        except Exception as e:
            logger.debug(f"Error checking for conflicts: {e}")
            # Silently continue - conflicts are not critical to work updates

    @staticmethod
    def _build_issue_description(description: str, commit_info) -> str:
        """Append commit details to a new issue description."""
        if not commit_info:
            return description
        commit_hash = commit_info.get("commit_hash", "")[:12]
        commit_message = commit_info.get("commit_message", "")
        if not (commit_hash or commit_message):
            return description
        return f"{description}\n\n---\n**Commit**: `{commit_hash}`\n**Message**: {commit_message}"

    async def _sync_to_azure(self, description, ticket_id=None, status=None, commit_info=None, workspace_settings=None):
        """Sync a work update or commit to Azure DevOps.

        workspace_settings: dict with per-workspace PM overrides (assignee,
            iteration_path, area_path). None = use empty values.

        Returns:
            Tuple of (azure_work_item_id: int | None, synced_platform: str | None)
        """
        if not self.azure_client or not config or not config.is_azure_sync_enabled():
            return None, None

        try:
            # Fetch current Azure work items
            azure_tasks = await self.task_repo.get_azure_tasks_async()
            if not azure_tasks:
                logger.debug("No Azure DevOps work items found for matching")
                # Optionally create a new work item if configured
                if config.is_azure_create_on_no_match() and description:
                    logger.info("Creating new Azure DevOps work item (no match, create_on_no_match enabled)")
                    ws = workspace_settings or {}
                    assignee = ws.get("assignee") or None
                    iteration_path = ws.get("iteration_path") or None
                    area_path = ws.get("area_path") or None
                    wi = await self.azure_client.create_work_item(
                        title=description[:200],
                        description=self._build_issue_description(description, commit_info),
                        assigned_to=assignee,
                        iteration_path=iteration_path,
                        area_path=area_path,
                    )
                    if wi:
                        logger.info(f"✓ Created Azure DevOps work item #{wi.id}")
                        return wi.id, "azure_devops"
                return None, None

            # Try to match by explicit ticket ID first, then fuzzy
            match_result = None
            threshold = config.get_azure_match_threshold()

            if ticket_id and self.task_matcher:
                match_result = self.task_matcher.match_task(
                    ticket_id, azure_tasks, threshold=threshold
                )

            if not match_result and self.task_matcher and description:
                match_result = self.task_matcher.match_task(
                    description, azure_tasks, threshold=threshold
                )

            if not match_result or match_result.confidence < threshold:
                logger.info(
                    f"No Azure work item matched above threshold "
                    f"({threshold}) for: {description[:80]}..."
                )
                if config.is_azure_create_on_no_match() and description:
                    logger.info("Creating new Azure DevOps work item (below threshold, create_on_no_match enabled)")
                    ws = workspace_settings or {}
                    assignee = ws.get("assignee") or None
                    iteration_path = ws.get("iteration_path") or None
                    area_path = ws.get("area_path") or None
                    wi = await self.azure_client.create_work_item(
                        title=description[:200],
                        description=self._build_issue_description(description, commit_info),
                        assigned_to=assignee,
                        iteration_path=iteration_path,
                        area_path=area_path,
                    )
                    if wi:
                        logger.info(f"✓ Created Azure DevOps work item #{wi.id}")
                        return wi.id, "azure_devops"
                return None, None

            work_item_id = int(match_result.task.id)
            logger.info(
                f"Matched Azure work item #{work_item_id}: "
                f"{match_result.task.title} "
                f"(confidence: {match_result.confidence:.0%}, "
                f"type: {match_result.match_type})"
            )

            # Add comment if auto_comment is enabled
            if config.is_azure_auto_comment():
                comment_parts = []
                if commit_info:
                    commit_hash = commit_info.get("commit_hash", "")[:12]
                    commit_msg = commit_info.get("commit_message", "")
                    author = commit_info.get("author", "")
                    comment_parts.append(
                        f"<b>Commit</b>: {commit_hash}<br/>"
                        f"<b>Author</b>: {author}<br/>"
                        f"<b>Message</b>: {commit_msg}"
                    )
                if description:
                    comment_parts.append(f"<b>Update</b>: {description}")
                if status:
                    comment_parts.append(f"<b>Status</b>: {status}")

                comment_text = "<br/>".join(comment_parts) if comment_parts else description
                success = await self.azure_client.add_comment(work_item_id, comment_text)
                if success:
                    logger.info(f"✓ Added comment to Azure work item #{work_item_id}")
                else:
                    logger.warning(f"Failed to add comment to Azure work item #{work_item_id}")

            # Auto-transition state if configured and status indicates done
            if (
                status
                and status.lower() in ("done", "completed", "closed", "resolved")
                and config.is_azure_auto_transition()
            ):
                done_state = config.get_azure_done_state()
                success = await self.azure_client.update_work_item_state(work_item_id, done_state)
                if success:
                    logger.info(f"✓ Transitioned Azure work item #{work_item_id} to '{done_state}'")
                else:
                    logger.warning(f"Failed to transition Azure work item #{work_item_id}")

            return work_item_id, "azure_devops"

        except Exception as e:
            logger.error(f"Error syncing to Azure DevOps: {e}")
            return None, None

    def _route_pm_sync(self, pm_platform, pm_project, description, ticket_id=None, status=None, commit_info=None, workspace_settings=None):
        """Route a PM sync call to the correct platform based on workspace config.

        When pm_platform is non-empty (set via workspaces.yaml), dispatches
        directly to that platform. Falls back to the legacy priority chain
        (Azure → GitLab → GitHub) when pm_platform is empty.

        workspace_settings: dict with per-workspace PM overrides (assignee,
            iteration_path, area_path, milestone). None = no override.

        Returns:
            Tuple of (work_item_id: int | None, synced_platform: str | None)
        """
        platform = (pm_platform or "").strip().lower()

        if platform == "none":
            logger.debug("Workspace pm_platform=none: skipping PM sync")
            return None, None

        if platform == "azure":
            logger.info(f"Workspace routing → Azure (pm_project={pm_project!r})")
            return self._run_azure_sync(description, ticket_id, status, commit_info, workspace_settings)

        if platform == "gitlab":
            logger.info(f"Workspace routing → GitLab (pm_project={pm_project!r})")
            return self._run_gitlab_sync(description, ticket_id, status, commit_info, workspace_settings)

        if platform == "github":
            logger.info(f"Workspace routing → GitHub")
            return self._run_github_sync(description, ticket_id, status, commit_info, workspace_settings)

        if platform == "jira":
            logger.info("Workspace routing → Jira (not yet implemented)")
            return None, None

        # Empty or unknown pm_platform → legacy priority chain
        if platform:
            logger.warning(f"Unknown pm_platform={platform!r}, falling back to priority chain")

        # Priority chain: Azure → GitLab → GitHub
        work_item_id, synced_platform = None, None

        if not work_item_id and self.azure_client and config and config.is_azure_sync_enabled():
            work_item_id, synced_platform = self._run_azure_sync(description, ticket_id, status, commit_info, workspace_settings)

        if not work_item_id and self.gitlab_client and config and config.is_gitlab_sync_enabled():
            work_item_id, synced_platform = self._run_gitlab_sync(description, ticket_id, status, commit_info, workspace_settings)

        if not work_item_id and self.github_client and config and config.is_github_sync_enabled():
            work_item_id, synced_platform = self._run_github_sync(description, ticket_id, status, commit_info, workspace_settings)

        return work_item_id, synced_platform

    def _run_azure_sync(self, description, ticket_id=None, status=None, commit_info=None, workspace_settings=None):
        """Run the async _sync_to_azure from synchronous handler code.

        Returns:
            Tuple of (azure_work_item_id: int | None, synced_platform: str | None)
        """
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            loop = None

        if loop and loop.is_running():
            # We are inside an existing event loop (shouldn't happen in bridge,
            # but be safe). Create a new thread with its own loop.
            import concurrent.futures
            with concurrent.futures.ThreadPoolExecutor(max_workers=1) as pool:
                future = pool.submit(
                    asyncio.run,
                    self._sync_to_azure(description, ticket_id, status, commit_info, workspace_settings),
                )
                return future.result()
        else:
            return asyncio.run(
                self._sync_to_azure(description, ticket_id, status, commit_info, workspace_settings)
            )

    async def _sync_to_gitlab(self, description, ticket_id=None, status=None, commit_info=None, workspace_settings=None):
        """Sync a work update or commit to GitLab.

        workspace_settings: dict with per-workspace PM overrides (milestone).
            None = no override.

        Returns:
            Tuple of (issue_iid: int | None, synced_platform: str | None)
        """
        if not self.gitlab_client or not config or not config.is_gitlab_sync_enabled():
            return None, None

        try:
            project_id = config.get_gitlab_default_project_id()

            # Fetch current GitLab issues assigned to user
            gitlab_issues = await self.gitlab_client.get_my_issues(state="opened")
            if not gitlab_issues:
                logger.debug("No GitLab issues found for matching")
                if config.is_gitlab_create_on_no_match() and description and project_id:
                    logger.info("Creating new GitLab issue (no match, create_on_no_match enabled)")
                    user_id = await self.gitlab_client._ensure_user_id()
                    ws = workspace_settings or {}
                    milestone_id = ws.get("milestone") or None
                    issue = await self.gitlab_client.create_issue(
                        project_id=project_id,
                        title=description[:200],
                        description=self._build_issue_description(description, commit_info),
                        labels=[config.get_gitlab_sync_label()],
                        assignee_ids=[user_id] if user_id else None,
                        milestone_id=milestone_id if milestone_id else None,
                    )
                    if issue:
                        logger.info(f"✓ Created GitLab issue #{issue.iid} in project {project_id}")
                        return issue.iid, "gitlab"
                return None, None

            # Convert issues to a format TaskMatcher can use
            # TaskMatcher expects objects with .id, .title attributes
            threshold = config.get_gitlab_match_threshold()
            match_result = None

            if ticket_id and self.task_matcher:
                match_result = self.task_matcher.match_task(
                    ticket_id, gitlab_issues, threshold=threshold
                )

            if not match_result and self.task_matcher and description:
                match_result = self.task_matcher.match_task(
                    description, gitlab_issues, threshold=threshold
                )

            if not match_result or match_result.confidence < threshold:
                logger.info(
                    f"No GitLab issue matched above threshold "
                    f"({threshold}) for: {description[:80]}..."
                )
                if config.is_gitlab_create_on_no_match() and description and project_id:
                    logger.info("Creating new GitLab issue (below threshold, create_on_no_match enabled)")
                    user_id = await self.gitlab_client._ensure_user_id()
                    ws = workspace_settings or {}
                    milestone_id = ws.get("milestone") or None
                    issue = await self.gitlab_client.create_issue(
                        project_id=project_id,
                        title=description[:200],
                        description=self._build_issue_description(description, commit_info),
                        labels=[config.get_gitlab_sync_label()],
                        assignee_ids=[user_id] if user_id else None,
                        milestone_id=milestone_id if milestone_id else None,
                    )
                    if issue:
                        logger.info(f"✓ Created GitLab issue #{issue.iid} in project {project_id}")
                        return issue.iid, "gitlab"
                return None, None

            matched_issue = match_result.task
            issue_iid = matched_issue.iid
            issue_project_id = matched_issue.project_id or project_id
            logger.info(
                f"Matched GitLab issue #{issue_iid}: "
                f"{matched_issue.title} "
                f"(confidence: {match_result.confidence:.0%}, "
                f"type: {match_result.match_type})"
            )

            # Auto-update issue description if configured
            if config.is_gitlab_auto_update_description() and commit_info and issue_project_id:
                commit_hash = commit_info.get("commit_hash", "")[:12]
                commit_msg = commit_info.get("commit_message", "")
                author = commit_info.get("author", "")
                existing_issue = await self.gitlab_client.get_issue(issue_project_id, issue_iid)
                if existing_issue:
                    existing_desc = existing_issue.description or ""
                    update_section = (
                        f"\n\n---\n**Latest commit**: `{commit_hash}` by {author}\n"
                        f"**Message**: {commit_msg}"
                    )
                    if commit_hash not in existing_desc:
                        new_desc = existing_desc + update_section
                        await self.gitlab_client.update_issue(
                            issue_project_id, issue_iid, {"description": new_desc}
                        )
                        logger.info(f"✓ Updated GitLab issue #{issue_iid} description")

            # Add comment if auto_comment is enabled
            if config.is_gitlab_auto_comment() and issue_project_id:
                comment_parts = []
                if commit_info:
                    commit_hash = commit_info.get("commit_hash", "")[:12]
                    commit_msg = commit_info.get("commit_message", "")
                    author = commit_info.get("author", "")
                    comment_parts.append(
                        f"**Commit**: {commit_hash}  \n"
                        f"**Author**: {author}  \n"
                        f"**Message**: {commit_msg}"
                    )
                if description:
                    comment_parts.append(f"**Update**: {description}")
                if status:
                    comment_parts.append(f"**Status**: {status}")

                comment_text = "\n\n".join(comment_parts) if comment_parts else description
                success = await self.gitlab_client.add_comment(issue_project_id, issue_iid, comment_text)
                if success:
                    logger.info(f"✓ Added comment to GitLab issue #{issue_iid}")
                else:
                    logger.warning(f"Failed to add comment to GitLab issue #{issue_iid}")

            # Auto-close issue if status indicates done
            if (
                status
                and status.lower() in ("done", "completed", "closed", "resolved")
                and config.is_gitlab_auto_transition()
                and issue_project_id
            ):
                success = await self.gitlab_client.close_issue(issue_project_id, issue_iid)
                if success:
                    logger.info(f"✓ Closed GitLab issue #{issue_iid}")
                else:
                    logger.warning(f"Failed to close GitLab issue #{issue_iid}")

            return issue_iid, "gitlab"

        except Exception as e:
            logger.error(f"Error syncing to GitLab: {e}")
            return None, None

    def _run_gitlab_sync(self, description, ticket_id=None, status=None, commit_info=None, workspace_settings=None):
        """Run the async _sync_to_gitlab from synchronous handler code.

        Returns:
            Tuple of (issue_iid: int | None, synced_platform: str | None)
        """
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            loop = None

        if loop and loop.is_running():
            import concurrent.futures
            with concurrent.futures.ThreadPoolExecutor(max_workers=1) as pool:
                future = pool.submit(
                    asyncio.run,
                    self._sync_to_gitlab(description, ticket_id, status, commit_info, workspace_settings),
                )
                return future.result()
        else:
            return asyncio.run(
                self._sync_to_gitlab(description, ticket_id, status, commit_info, workspace_settings)
            )

    async def _sync_to_github(self, description, ticket_id=None, status=None, commit_info=None, workspace_settings=None):
        """Sync a work update or commit to GitHub.

        workspace_settings: dict with per-workspace PM overrides (milestone).
            None = no override.

        Returns:
            Tuple of (issue_number: int | None, synced_platform: str | None)
        """
        if not self.github_client or not config or not config.is_github_sync_enabled():
            return None, None

        try:
            # Fetch open GitHub issues assigned to the user
            github_issues = await self.github_client.get_my_issues(state="open")
            if not github_issues:
                logger.debug("No GitHub issues found for matching")
                if config.is_github_create_on_no_match() and description:
                    logger.info("Creating new GitHub issue (no match, create_on_no_match enabled)")
                    login = await self.github_client._ensure_login()
                    ws = workspace_settings or {}
                    issue = await self.github_client.create_issue(
                        title=description[:200],
                        body=self._build_issue_description(description, commit_info),
                        labels=[config.get_github_sync_label()],
                        assignees=[login] if login else None,
                        milestone=ws.get("milestone") or None,
                    )
                    if issue:
                        logger.info(f"✓ Created GitHub issue #{issue.number}")
                        return issue.number, "github"
                return None, None

            threshold = config.get_github_match_threshold()
            match_result = None

            if ticket_id and self.task_matcher:
                match_result = self.task_matcher.match_task(
                    ticket_id, github_issues, threshold=threshold
                )

            if not match_result and self.task_matcher and description:
                match_result = self.task_matcher.match_task(
                    description, github_issues, threshold=threshold
                )

            if not match_result or match_result.confidence < threshold:
                logger.info(
                    f"No GitHub issue matched above threshold "
                    f"({threshold}) for: {description[:80]}..."
                )
                if config.is_github_create_on_no_match() and description:
                    logger.info("Creating new GitHub issue (below threshold, create_on_no_match enabled)")
                    login = await self.github_client._ensure_login()
                    ws = workspace_settings or {}
                    issue = await self.github_client.create_issue(
                        title=description[:200],
                        body=self._build_issue_description(description, commit_info),
                        labels=[config.get_github_sync_label()],
                        assignees=[login] if login else None,
                        milestone=ws.get("milestone") or None,
                    )
                    if issue:
                        logger.info(f"✓ Created GitHub issue #{issue.number}")
                        return issue.number, "github"
                return None, None

            matched_issue = match_result.task
            issue_number = matched_issue.number
            logger.info(
                f"Matched GitHub issue #{issue_number}: "
                f"{matched_issue.title} "
                f"(confidence: {match_result.confidence:.0%}, "
                f"type: {match_result.match_type})"
            )

            # Auto-update issue description if configured
            if config.is_github_auto_update_description() and commit_info:
                commit_hash = commit_info.get("commit_hash", "")[:12]
                commit_msg = commit_info.get("commit_message", "")
                author = commit_info.get("author", "")
                existing_issue = await self.github_client.get_issue(issue_number)
                if existing_issue:
                    existing_body = existing_issue.body or ""
                    update_section = (
                        f"\n\n---\n**Latest commit**: `{commit_hash}` by {author}\n"
                        f"**Message**: {commit_msg}"
                    )
                    if commit_hash not in existing_body:
                        new_body = existing_body + update_section
                        await self.github_client.update_issue(issue_number, body=new_body)
                        logger.info(f"✓ Updated GitHub issue #{issue_number} description")

            # Add comment if auto_comment is enabled
            if config.is_github_auto_comment():
                comment_parts = []
                if commit_info:
                    commit_hash = commit_info.get("commit_hash", "")[:12]
                    commit_msg = commit_info.get("commit_message", "")
                    author = commit_info.get("author", "")
                    comment_parts.append(
                        f"**Commit**: {commit_hash}  \n"
                        f"**Author**: {author}  \n"
                        f"**Message**: {commit_msg}"
                    )
                if description:
                    comment_parts.append(f"**Update**: {description}")
                if status:
                    comment_parts.append(f"**Status**: {status}")

                comment_text = "\n\n".join(comment_parts) if comment_parts else description
                success = await self.github_client.add_comment(issue_number, comment_text)
                if success:
                    logger.info(f"✓ Added comment to GitHub issue #{issue_number}")
                else:
                    logger.warning(f"Failed to add comment to GitHub issue #{issue_number}")

            # Auto-close if status indicates done
            if (
                status
                and status.lower() in ("done", "completed", "closed", "resolved")
                and config.is_github_auto_transition()
            ):
                success = await self.github_client.close_issue(issue_number)
                if success:
                    logger.info(f"✓ Closed GitHub issue #{issue_number}")
                else:
                    logger.warning(f"Failed to close GitHub issue #{issue_number}")

            return issue_number, "github"

        except Exception as e:
            logger.error(f"Error syncing to GitHub: {e}")
            return None, None

    def _run_github_sync(self, description, ticket_id=None, status=None, commit_info=None, workspace_settings=None):
        """Run the async _sync_to_github from synchronous handler code.

        Returns:
            Tuple of (issue_number: int | None, synced_platform: str | None)
        """
        try:
            loop = asyncio.get_running_loop()
        except RuntimeError:
            loop = None

        if loop and loop.is_running():
            import concurrent.futures
            with concurrent.futures.ThreadPoolExecutor(max_workers=1) as pool:
                future = pool.submit(
                    asyncio.run,
                    self._sync_to_github(description, ticket_id, status, commit_info, workspace_settings),
                )
                return future.result()
        else:
            return asyncio.run(
                self._sync_to_github(description, ticket_id, status, commit_info, workspace_settings)
            )

    def handle_workspace_reload(self, msg: IPCMessage):
        """Handle workspace_reload message from Go daemon.

        The Go daemon reloads workspaces.yaml itself; the Python side only
        needs to acknowledge the request and log that a reload was requested.
        Future work: re-initialise per-workspace PM clients here.
        """
        logger.info("Workspace reload requested by Go daemon")
        ack = create_ack_message(msg.id)
        self.ipc_client.send_message(ack)
        logger.debug("Sent ACK for workspace_reload")

    def handle_status_query(self, msg: IPCMessage):
        """Handle status query from Go daemon"""
        logger.info("📊 Status query received")
        
        response = create_response_message(
            request_id=msg.id,
            data={
                "status": "running",
                "triggers_processed": sum(self.trigger_count.values()),
                "commit_triggers": self.trigger_count["commit"],
                "timer_triggers": self.trigger_count["timer"],
                "python_version": sys.version,
                "uptime_seconds": time.time() - self.start_time
            }
        )
        self.ipc_client.send_message(response)
    
    def handle_shutdown(self, msg: IPCMessage):
        """Handle shutdown message from Go daemon"""
        logger.info("🛑 Shutdown message received")
        # Close async Azure client session
        if self.azure_client:
            try:
                asyncio.run(self.azure_client.close())
            except Exception:
                pass
        if self.gitlab_client:
            try:
                asyncio.run(self.gitlab_client.close())
            except Exception:
                pass
        if self.github_client:
            try:
                asyncio.run(self.github_client.close())
            except Exception:
                pass
        self.running = False
    
    def handle_report_trigger(self, msg: IPCMessage):
        """Handle report trigger from Go daemon or CLI"""
        logger.info("="* 60)
        logger.info("📊 REPORT TRIGGER RECEIVED")
        logger.info("=" * 60)
        
        # Send acknowledgment
        ack = create_ack_message(msg.id)
        self.ipc_client.send_message(ack)
        
        data = msg.data or {}
        date_str = data.get('date')  # Optional: specific date YYYY-MM-DD
        report_type = data.get('type', 'daily')  # 'daily' or 'weekly'
        save = data.get('save', True)
        
        # Parse date if provided
        from datetime import datetime
        if date_str:
            try:
                datetime.strptime(date_str, '%Y-%m-%d')
                logger.info(f"Generating report for: {date_str}")
            except ValueError:
                logger.warning(f"Invalid date format: {date_str}, using today")
        
        # Generate the appropriate report
        if report_type == 'weekly':
            logger.info("Generating weekly report...")
            self._generate_weekly_report(save=save)
        else:
            logger.info("Generating daily report...")
            self._generate_daily_report(save=save)
    
    def start(self):
        """Start the Python bridge"""
        self.start_time = time.time()
        
        logger.info("🚀 DevTrack Python Bridge Starting...")
        logger.info("=" * 60)
        
        # Register handlers
        self.ipc_client.register_handler(MessageType.COMMIT_TRIGGER, self.handle_commit_trigger)
        self.ipc_client.register_handler(MessageType.TIMER_TRIGGER, self.handle_timer_trigger)
        self.ipc_client.register_handler(MessageType.REPORT_TRIGGER, self.handle_report_trigger)
        self.ipc_client.register_handler(MessageType.STATUS_QUERY, self.handle_status_query)
        self.ipc_client.register_handler(MessageType.SHUTDOWN, self.handle_shutdown)
        self.ipc_client.register_handler(MessageType.WORKSPACE_RELOAD, self.handle_workspace_reload)
        
        # Connect to IPC server
        logger.info("Connecting to Go daemon IPC server...")
        if not self.ipc_client.connect(timeout=5, retry_count=5):
            logger.error("❌ Failed to connect to IPC server")
            logger.error("   Make sure the Go daemon is running: ./devtrack start")
            return 1
        
        logger.info("✅ Connected to IPC server")
        logger.info("")
        logger.info("🎧 Listening for triggers from Go daemon...")
        logger.info("   - Commit triggers (Git activity)")
        logger.info("   - Timer triggers (scheduled prompts)")
        logger.info("")
        logger.info("Press Ctrl+C to stop")
        logger.info("=" * 60)
        logger.info("")
        
        # Start listening
        self.running = True
        self.ipc_client.start_listening()
        
        # Keep running until shutdown
        try:
            while self.running:
                time.sleep(1)
        except KeyboardInterrupt:
            logger.info("\n⚠️  Interrupted by user")
        finally:
            logger.info("Shutting down...")
            self.ipc_client.stop_listening()
            self.ipc_client.disconnect()
            logger.info("✓ Python bridge stopped")
        
        return 0


def main():
    """Main entry point"""
    bridge = DevTrackBridge()
    return bridge.start()


if __name__ == "__main__":
    sys.exit(main())
