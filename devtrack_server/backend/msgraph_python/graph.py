# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# <UserAuthConfigSnippet>
from configparser import SectionProxy
from datetime import datetime, timezone
from typing import Optional

from azure.identity import DeviceCodeCredential
try:
    from azure.identity import TokenCachePersistenceOptions
    _PERSISTENT_CACHE_AVAILABLE = True
except ImportError:
    _PERSISTENT_CACHE_AVAILABLE = False

from msgraph import GraphServiceClient
from msgraph.generated.users.item.user_item_request_builder import UserItemRequestBuilder
from msgraph.generated.users.item.mail_folders.item.messages.messages_request_builder import (
    MessagesRequestBuilder)
from msgraph.generated.users.item.send_mail.send_mail_post_request_body import (
    SendMailPostRequestBody)
from msgraph.generated.models.message import Message
from msgraph.generated.models.item_body import ItemBody
from msgraph.generated.models.body_type import BodyType
from msgraph.generated.models.recipient import Recipient
from msgraph.generated.models.email_address import EmailAddress
from msgraph.generated.chats.chats_request_builder import ChatsRequestBuilder
from msgraph.generated.chats.item.messages.messages_request_builder import MessagesRequestBuilder as ChatMessagesRequestBuilder

class Graph:
    settings: SectionProxy
    device_code_credential: DeviceCodeCredential
    user_client: GraphServiceClient

    def __init__(self, config: SectionProxy):
        self.settings = config
        client_id = self.settings['clientId']
        tenant_id = self.settings['tenantId']
        graph_scopes = self.settings['graphUserScopes'].split(' ')

        if _PERSISTENT_CACHE_AVAILABLE:
            cache_options = TokenCachePersistenceOptions(
                name="devtrack",
                allow_unencrypted_storage=True  # fallback for Linux without system keyring
            )
            self.device_code_credential = DeviceCodeCredential(
                client_id,
                tenant_id=tenant_id,
                cache_persistence_options=cache_options
            )
        else:
            self.device_code_credential = DeviceCodeCredential(client_id, tenant_id=tenant_id)

        self.user_client = GraphServiceClient(self.device_code_credential, graph_scopes)
# </UserAuthConfigSnippet>

    # <GetUserTokenSnippet>
    async def get_user_token(self):
        graph_scopes = self.settings['graphUserScopes']
        access_token = self.device_code_credential.get_token(graph_scopes)
        return access_token.token
    # </GetUserTokenSnippet>

    # <GetUserSnippet>
    async def get_user(self):
        # Only request specific properties using $select
        query_params = UserItemRequestBuilder.UserItemRequestBuilderGetQueryParameters(
            select=['id', 'displayName', 'mail', 'userPrincipalName']
        )

        request_config = UserItemRequestBuilder.UserItemRequestBuilderGetRequestConfiguration(
            query_parameters=query_params
        )

        user = await self.user_client.me.get(request_configuration=request_config)
        return user
    # </GetUserSnippet>

    # <GetInboxSnippet>
    async def get_inbox(self):
        query_params = MessagesRequestBuilder.MessagesRequestBuilderGetQueryParameters(
            # Only request specific properties
            select=['from', 'isRead', 'receivedDateTime', 'subject'],
            # Get at most 25 results
            top=25,
            # Sort by received time, newest first
            orderby=['receivedDateTime DESC']
        )
        request_config = MessagesRequestBuilder.MessagesRequestBuilderGetRequestConfiguration(
            query_parameters= query_params
        )

        messages = await self.user_client.me.mail_folders.by_mail_folder_id('inbox').messages.get(
                request_configuration=request_config)
        return messages
    # </GetInboxSnippet>

    # <GetTeamsChatsSnippet>
    async def get_teams_chats(self):
        query_params = ChatsRequestBuilder.ChatsRequestBuilderGetQueryParameters(
            # Only request specific properties
            select=['id', 'topic', 'chatType', 'createdDateTime', 'lastUpdatedDateTime'],
            # Get at most 50 results (increased since we can't sort)
            top=50
        )
        request_config = ChatsRequestBuilder.ChatsRequestBuilderGetRequestConfiguration(
            query_parameters=query_params
        )

        chats = await self.user_client.chats.get(request_configuration=request_config)
        
        # Sort manually after retrieving data
        if chats and chats.value:
            chats.value.sort(key=lambda x: x.last_updated_date_time if x.last_updated_date_time else x.created_date_time, reverse=True)
        
        return chats
    # </GetTeamsChatsSnippet>

    # <GetChatMessagesSnippet>
    async def get_chat_messages(self, chat_id: str):
        query_params = ChatMessagesRequestBuilder.MessagesRequestBuilderGetQueryParameters(
            # Get at most 50 results
            top=50
        )
        request_config = ChatMessagesRequestBuilder.MessagesRequestBuilderGetRequestConfiguration(
            query_parameters=query_params
        )

        messages = await self.user_client.chats.by_chat_id(chat_id).messages.get(
            request_configuration=request_config)
        return messages
    # </GetChatMessagesSnippet>

    # <SendMailSnippet>
    async def send_mail(self, subject: str, body: str, recipient: str):
        message = Message()
        message.subject = subject

        message.body = ItemBody()
        message.body.content_type = BodyType.Text
        message.body.content = body

        to_recipient = Recipient()
        to_recipient.email_address = EmailAddress()
        to_recipient.email_address.address = recipient
        message.to_recipients = []
        message.to_recipients.append(to_recipient)

        request_body = SendMailPostRequestBody()
        request_body.message = message

        await self.user_client.me.send_mail.post(body=request_body)
    # </SendMailSnippet>

    # <GetOneOnOneChatsWithPersonSnippet>
    async def get_one_on_one_chats_with_person(self, person_name: str = None, person_email: str = None):
        # Get all chats first
        chats = await self.get_teams_chats()
        one_on_one_chats = []
        
        if chats and chats.value:
            for chat in chats.value:
                # Only look at one-on-one chats
                if chat.chat_type == 'oneOnOne':
                    try:
                        members = await self.user_client.chats.by_chat_id(chat.id).members.get()
                        if members and members.value:
                            for member in members.value:
                                if hasattr(member, 'display_name') and member.display_name:
                                    if person_name and person_name.lower() in member.display_name.lower():
                                        one_on_one_chats.append(chat)
                                        break
                                if hasattr(member, 'email') and member.email:
                                    if person_email and person_email.lower() == member.email.lower():
                                        one_on_one_chats.append(chat)
                                        break
                    except Exception as e:
                        continue
        
        return one_on_one_chats
    # </GetOneOnOneChatsWithPersonSnippet>

    # <GetAllChatMessagesSnippet>
    async def get_all_chat_messages(self, chat_id: str, since_datetime: Optional[datetime] = None):
        """
        Fetch all messages from a chat, optionally stopping when messages older
        than since_datetime are reached (delta/incremental fetch).

        Args:
            chat_id: Teams chat ID
            since_datetime: If provided, stop paginating once messages are older
                            than this timestamp (must be timezone-aware).
        """
        all_messages = []
        next_link = None

        try:
            while True:
                if next_link:
                    response = await self.user_client.chats.by_chat_id(chat_id).messages.with_url(next_link).get()
                else:
                    query_params = ChatMessagesRequestBuilder.MessagesRequestBuilderGetQueryParameters(
                        top=50
                    )
                    request_config = ChatMessagesRequestBuilder.MessagesRequestBuilderGetRequestConfiguration(
                        query_parameters=query_params
                    )
                    response = await self.user_client.chats.by_chat_id(chat_id).messages.get(
                        request_configuration=request_config)

                if response and response.value:
                    batch = response.value
                    all_messages.extend(batch)
                    print(f"Fetched {len(batch)} messages... (Total: {len(all_messages)})")

                    # Early termination: API returns newest-first. If the oldest
                    # message in this batch is older than the cutoff, stop paging.
                    if since_datetime is not None:
                        oldest_in_batch = min(
                            (m.created_date_time for m in batch if m.created_date_time),
                            default=None
                        )
                        if oldest_in_batch is not None:
                            # Ensure both are timezone-aware for comparison
                            cutoff = since_datetime
                            if cutoff.tzinfo is None:
                                cutoff = cutoff.replace(tzinfo=timezone.utc)
                            if oldest_in_batch.tzinfo is None:
                                oldest_in_batch = oldest_in_batch.replace(tzinfo=timezone.utc)
                            if oldest_in_batch <= cutoff:
                                break

                if hasattr(response, 'odata_next_link') and response.odata_next_link:
                    next_link = response.odata_next_link
                else:
                    break

        except Exception as e:
            print(f"Error fetching messages: {e}")

        # Sort oldest-first for processing
        if all_messages:
            all_messages.sort(key=lambda x: x.created_date_time if x.created_date_time else datetime.min)

        return all_messages
    # </GetAllChatMessagesSnippet>

    # <GetChatsWithPersonSnippet>
    async def get_chats_with_person(self, person_name: str = None, person_email: str = None):
        # Get all chats first
        chats = await self.get_teams_chats()
        matching_chats = []
        
        if chats and chats.value:
            for chat in chats.value:
                # Get chat members to check if the person is in the chat
                try:
                    members = await self.user_client.chats.by_chat_id(chat.id).members.get()
                    if members and members.value:
                        for member in members.value:
                            if hasattr(member, 'display_name') and member.display_name:
                                if person_name and person_name.lower() in member.display_name.lower():
                                    matching_chats.append(chat)
                                    break
                            if hasattr(member, 'email') and member.email:
                                if person_email and person_email.lower() == member.email.lower():
                                    matching_chats.append(chat)
                                    break
                except Exception as e:
                    # Skip chats where we can't get members (permissions issue)
                    continue
        
        return matching_chats
    # </GetChatsWithPersonSnippet>

    # <GetChatsAddressedToMeSnippet>
    async def get_chats_addressed_to_me(self):
        # Get current user info
        current_user = await self.get_user()
        current_user_email = current_user.mail or current_user.user_principal_name
        
        # Get all chats
        chats = await self.get_teams_chats()
        addressed_chats = []
        
        if chats and chats.value:
            for chat in chats.value:
                try:
                    # Get recent messages from this chat
                    messages = await self.get_chat_messages(chat.id)
                    if messages and messages.value:
                        # Check if any recent message mentions the current user
                        for message in messages.value:
                            if message.body and message.body.content:
                                content = message.body.content.lower()
                                user_name = current_user.display_name.lower() if current_user.display_name else ""
                                
                                # Check if user is mentioned by name or email
                                if (user_name and user_name in content) or \
                                   (current_user_email and current_user_email.lower() in content) or \
                                   f"@{current_user.display_name}" in message.body.content if current_user.display_name else False:
                                    addressed_chats.append({
                                        'chat': chat,
                                        'mentioning_message': message
                                    })
                                    break
                except Exception as e:
                    # Skip chats where we can't get messages
                    continue
        
        return addressed_chats
    # </GetChatsAddressedToMeSnippet>

    # <SearchChatsSnippet>
    async def search_chats_by_keyword(self, keyword: str):
        # Get all chats
        chats = await self.get_teams_chats()
        matching_chats = []
        
        if chats and chats.value:
            for chat in chats.value:
                try:
                    # Check chat topic first
                    if chat.topic and keyword.lower() in chat.topic.lower():
                        matching_chats.append({
                            'chat': chat,
                            'match_type': 'topic',
                            'match_text': chat.topic
                        })
                        continue
                    
                    # Search in recent messages
                    messages = await self.get_chat_messages(chat.id)
                    if messages and messages.value:
                        for message in messages.value:
                            if message.body and message.body.content:
                                if keyword.lower() in message.body.content.lower():
                                    matching_chats.append({
                                        'chat': chat,
                                        'match_type': 'message',
                                        'match_text': message.body.content[:100] + "..." if len(message.body.content) > 100 else message.body.content
                                    })
                                    break
                except Exception as e:
                    continue
        
        return matching_chats
    # </SearchChatsSnippet>

    # <MakeGraphCallSnippet>
    async def make_graph_call(self):
        # Example: Get user's joined teams
        teams = await self.user_client.me.joined_teams.get()
        if teams and teams.value:
            print("User's Teams:")
            for team in teams.value:
                print(f"  - {team.display_name} (ID: {team.id})")
        return teams
    # </MakeGraphCallSnippet>