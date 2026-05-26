# Copyright (c) Microsoft Corporation.
# Licensed under the MIT License.

# <ProgramSnippet>
import asyncio
import configparser
import duckdb
import os
from datetime import datetime
from msgraph.generated.models.o_data_errors.o_data_error import ODataError
from graph import Graph

async def main():
    print('Python Graph Tutorial with Teams Integration\n')

    # Load settings
    config = configparser.ConfigParser()
    config.read(['config.cfg', 'config.dev.cfg'])
    azure_settings = config['azure']

    graph: Graph = Graph(azure_settings)

    await greet_user(graph)

    choice = -1

    while choice != 0:
        print('Please choose one of the following options:')
        print('0. Exit')
        print('1. Display access token')
        print('2. List my inbox')
        print('3. Send mail')
        print('4. List Teams chats')
        print('5. View chat messages')
        print('6. Find chats with specific person')
        print('7. List chats addressed to me')
        print('8. Search chats by keyword')
        print('9. Export one-on-one chat to database')
        print('10. Make a Graph call')

        try:
            choice = int(input())
        except ValueError:
            choice = -1

        try:
            if choice == 0:
                print('Goodbye...')
            elif choice == 1:
                await display_access_token(graph)
            elif choice == 2:
                await list_inbox(graph)
            elif choice == 3:
                await send_mail(graph)
            elif choice == 4:
                await list_teams_chats(graph)
            elif choice == 5:
                await view_chat_messages(graph)
            elif choice == 6:
                await find_chats_with_person(graph)
            elif choice == 7:
                await list_chats_addressed_to_me(graph)
            elif choice == 8:
                await search_chats_by_keyword(graph)
            elif choice == 9:
                await export_one_on_one_chat(graph)
            elif choice == 10:
                await make_graph_call(graph)
            else:
                print('Invalid choice!\n')
        except ODataError as odata_error:
            print('Error:')
            if odata_error.error:
                print(odata_error.error.code, odata_error.error.message)
# </ProgramSnippet>

# <GreetUserSnippet>
async def greet_user(graph: Graph):
    user = await graph.get_user()
    if user:
        print('Hello,', user.display_name)
        # For Work/school accounts, email is in mail property
        # Personal accounts, email is in userPrincipalName
        print('Email:', user.mail or user.user_principal_name, '\n')
# </GreetUserSnippet>

# <DisplayAccessTokenSnippet>
async def display_access_token(graph: Graph):
    token = await graph.get_user_token()
    print('User token:', token, '\n')
# </DisplayAccessTokenSnippet>

# <ListInboxSnippet>
async def list_inbox(graph: Graph):
    message_page = await graph.get_inbox()
    if message_page and message_page.value:
        # Output each message's details
        for message in message_page.value:
            print('Message:', message.subject)
            if (
                message.from_ and
                message.from_.email_address
            ):
                print('  From:', message.from_.email_address.name or 'NONE')
            else:
                print('  From: NONE')
            print('  Status:', 'Read' if message.is_read else 'Unread')
            print('  Received:', message.received_date_time)

        # If @odata.nextLink is present
        more_available = message_page.odata_next_link is not None
        print('\nMore messages available?', more_available, '\n')
# </ListInboxSnippet>

# <ListTeamsChatsSnippet>
async def list_teams_chats(graph: Graph):
    chats_page = await graph.get_teams_chats()
    if chats_page and chats_page.value:
        print('Your Teams Chats:')
        for chat in chats_page.value:
            chat_topic = chat.topic if chat.topic else f"Chat {chat.id[:8]}..."
            print(f'  Chat: {chat_topic}')
            print(f'    ID: {chat.id}')
            print(f'    Type: {chat.chat_type}')
            print(f'    Created: {chat.created_date_time}')
            print(f'    Last Updated: {chat.last_updated_date_time}')
            print()
        
        more_available = chats_page.odata_next_link is not None
        print(f'More chats available? {more_available}\n')
    else:
        print('No chats found.\n')
# </ListTeamsChatsSnippet>

# <ViewChatMessagesSnippet>
async def view_chat_messages(graph: Graph):
    # First, get chats to show available options
    chats_page = await graph.get_teams_chats()
    if chats_page and chats_page.value:
        print('Available chats:')
        for i, chat in enumerate(chats_page.value[:10]):  # Show first 10 chats
            chat_topic = chat.topic if chat.topic else f"Chat {chat.id[:8]}..."
            print(f'  {i + 1}. {chat_topic}')
        
        try:
            choice = int(input('Select a chat (number): ')) - 1
            if 0 <= choice < len(chats_page.value):
                selected_chat = chats_page.value[choice]
                print(f'\nFetching messages for: {selected_chat.topic or "Unnamed Chat"}')
                
                messages_page = await graph.get_chat_messages(selected_chat.id)
                if messages_page and messages_page.value:
                    print('\nRecent messages:')
                    for message in messages_page.value:
                        sender = "Unknown"
                        if message.from_ and hasattr(message.from_, 'user') and message.from_.user:
                            sender = message.from_.user.display_name or "Unknown"
                        
                        print(f'  From: {sender}')
                        print(f'  Time: {message.created_date_time}')
                        if message.body and message.body.content:
                            content = message.body.content[:100] + "..." if len(message.body.content) > 100 else message.body.content
                            print(f'  Message: {content}')
                        print()
                else:
                    print('No messages found in this chat.\n')
            else:
                print('Invalid selection.\n')
        except (ValueError, IndexError):
            print('Invalid input.\n')
    else:
        print('No chats available.\n')
# </ViewChatMessagesSnippet>

# <SendMailSnippet>
async def send_mail(graph: Graph):
    # Send mail to the signed-in user
    # Get the user for their email address
    user = await graph.get_user()
    if user:
        user_email = user.mail or user.user_principal_name

        await graph.send_mail('Testing Microsoft Graph', 'Hello world!', user_email or '')
        print('Mail sent.\n')
# </SendMailSnippet>

# <FindChatsWithPersonSnippet>
async def find_chats_with_person(graph: Graph):
    print('Search for chats with a specific person:')
    print('1. Search by name')
    print('2. Search by email')
    
    try:
        search_type = int(input('Choose search type (1 or 2): '))
        
        if search_type == 1:
            person_name = input('Enter person\'s name (or part of it): ').strip()
            if person_name:
                print(f'\nSearching for chats with "{person_name}"...')
                chats = await graph.get_chats_with_person(person_name=person_name)
            else:
                print('Name cannot be empty.\n')
                return
        elif search_type == 2:
            person_email = input('Enter person\'s email: ').strip()
            if person_email:
                print(f'\nSearching for chats with "{person_email}"...')
                chats = await graph.get_chats_with_person(person_email=person_email)
            else:
                print('Email cannot be empty.\n')
                return
        else:
            print('Invalid choice.\n')
            return
        
        if chats:
            print(f'Found {len(chats)} chat(s):')
            for i, chat in enumerate(chats, 1):
                chat_topic = chat.topic if chat.topic else f"Chat {chat.id[:8]}..."
                print(f'  {i}. {chat_topic}')
                print(f'     Type: {chat.chat_type}')
                print(f'     Last Updated: {chat.last_updated_date_time}')
                print()
        else:
            print('No chats found with that person.\n')
            
    except ValueError:
        print('Invalid input.\n')
# </FindChatsWithPersonSnippet>

# <ListChatsAddressedToMeSnippet>
async def list_chats_addressed_to_me(graph: Graph):
    print('Searching for chats where you are mentioned...')
    
    addressed_chats = await graph.get_chats_addressed_to_me()
    
    if addressed_chats:
        print(f'\nFound {len(addressed_chats)} chat(s) where you are mentioned:')
        for i, item in enumerate(addressed_chats, 1):
            chat = item['chat']
            message = item['mentioning_message']
            
            chat_topic = chat.topic if chat.topic else f"Chat {chat.id[:8]}..."
            print(f'  {i}. {chat_topic}')
            print(f'     Type: {chat.chat_type}')
            print(f'     Last mention: {message.created_date_time}')
            
            # Show snippet of the mentioning message
            if message.body and message.body.content:
                snippet = message.body.content[:100] + "..." if len(message.body.content) > 100 else message.body.content
                print(f'     Message snippet: "{snippet}"')
            print()
    else:
        print('No chats found where you are specifically mentioned.\n')
# </ListChatsAddressedToMeSnippet>

# <SearchChatsByKeywordSnippet>
async def search_chats_by_keyword(graph: Graph):
    keyword = input('Enter keyword to search for: ').strip()
    
    if not keyword:
        print('Keyword cannot be empty.\n')
        return
    
    print(f'\nSearching for chats containing "{keyword}"...')
    
    matching_chats = await graph.search_chats_by_keyword(keyword)
    
    if matching_chats:
        print(f'Found {len(matching_chats)} chat(s) containing "{keyword}":')
        for i, item in enumerate(matching_chats, 1):
            chat = item['chat']
            match_type = item['match_type']
            match_text = item['match_text']
            
            chat_topic = chat.topic if chat.topic else f"Chat {chat.id[:8]}..."
            print(f'  {i}. {chat_topic}')
            print(f'     Type: {chat.chat_type}')
            print(f'     Match found in: {match_type}')
            print(f'     Match text: "{match_text}"')
            print(f'     Last Updated: {chat.last_updated_date_time}')
            print()
    else:
        print(f'No chats found containing "{keyword}".\n')
# </SearchChatsByKeywordSnippet>

# <ExportOneOnOneChatSnippet>
async def export_one_on_one_chat(graph: Graph):
    print('Export one-on-one chat to DuckDB database:')
    print('1. Search by name')
    print('2. Search by email')
    
    try:
        search_type = int(input('Choose search type (1 or 2): '))
        
        if search_type == 1:
            person_name = input('Enter person\'s name (or part of it): ').strip()
            if person_name:
                print(f'\nSearching for one-on-one chats with "{person_name}"...')
                chats = await graph.get_one_on_one_chats_with_person(person_name=person_name)
            else:
                print('Name cannot be empty.\n')
                return
        elif search_type == 2:
            person_email = input('Enter person\'s email: ').strip()
            if person_email:
                print(f'\nSearching for one-on-one chats with "{person_email}"...')
                chats = await graph.get_one_on_one_chats_with_person(person_email=person_email)
            else:
                print('Email cannot be empty.\n')
                return
        else:
            print('Invalid choice.\n')
            return
        
        if not chats:
            print('No one-on-one chats found with that person.\n')
            return
        
        if len(chats) > 1:
            print(f'Found {len(chats)} one-on-one chat(s):')
            for i, chat in enumerate(chats, 1):
                chat_topic = chat.topic if chat.topic else f"Chat {chat.id[:8]}..."
                print(f'  {i}. {chat_topic}')
                print(f'     Created: {chat.created_date_time}')
                print(f'     Last Updated: {chat.last_updated_date_time}')
                print()
            
            try:
                choice = int(input('Select a chat to export (number): ')) - 1
                if 0 <= choice < len(chats):
                    selected_chat = chats[choice]
                else:
                    print('Invalid selection.\n')
                    return
            except ValueError:
                print('Invalid input.\n')
                return
        else:
            selected_chat = chats[0]
            chat_topic = selected_chat.topic if selected_chat.topic else f"Chat {selected_chat.id[:8]}..."
            print(f'Found one chat: {chat_topic}')
        
        # Confirm export
        confirmation = input('\nDo you want to export this chat to DuckDB? (y/n): ').strip().lower()
        if confirmation != 'y':
            print('Export cancelled.\n')
            return
        
        print('\nFetching all messages from the chat... This may take a while.')
        messages = await graph.get_all_chat_messages(selected_chat.id)
        
        if not messages:
            print('No messages found in this chat.\n')
            return
        
        # Prepare database filename
        timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
        person_identifier = person_name if search_type == 1 else person_email.split('@')[0]
        db_filename = f"teams_chat_{person_identifier}_{timestamp}.duckdb"
        
        print(f'Exporting {len(messages)} messages to {db_filename}...')
        
        # Create DuckDB database and table
        conn = duckdb.connect(db_filename)
        
        # Create table
        conn.execute("""
            CREATE TABLE chat_messages (
                message_id VARCHAR,
                chat_id VARCHAR,
                sender_name VARCHAR,
                sender_email VARCHAR,
                content TEXT,
                created_datetime TIMESTAMP,
                message_type VARCHAR,
                is_deleted BOOLEAN
            )
        """)
        
        # Prepare data for insertion
        message_data = []
        for message in messages:
            sender_name = "Unknown"
            sender_email = "Unknown"
            
            # Handle different sender object structures
            if message.from_:
                if hasattr(message.from_, 'user') and message.from_.user:
                    # Standard user object
                    sender_name = message.from_.user.display_name or "Unknown"
                    if hasattr(message.from_.user, 'user_principal_name'):
                        sender_email = message.from_.user.user_principal_name or "Unknown"
                    elif hasattr(message.from_.user, 'email'):
                        sender_email = message.from_.user.email or "Unknown"
                elif hasattr(message.from_, 'display_name'):
                    # Direct display name on from object
                    sender_name = message.from_.display_name or "Unknown"
                elif hasattr(message.from_, 'id'):
                    # Fallback to ID if available
                    sender_name = f"User_{message.from_.id[:8]}" if message.from_.id else "Unknown"
            
            content = ""
            if message.body and message.body.content:
                content = message.body.content
            
            is_deleted = hasattr(message, 'deleted_date_time') and message.deleted_date_time is not None
            
            message_data.append((
                message.id or "Unknown",
                selected_chat.id,
                sender_name,
                sender_email,
                content,
                message.created_date_time,
                str(message.message_type) if hasattr(message, 'message_type') and message.message_type else "message",
                is_deleted
            ))
        
        # Insert all messages
        conn.executemany("""
            INSERT INTO chat_messages 
            (message_id, chat_id, sender_name, sender_email, content, created_datetime, message_type, is_deleted)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        """, message_data)
        
        # Create indexes for better query performance
        conn.execute("CREATE INDEX idx_created_datetime ON chat_messages(created_datetime)")
        conn.execute("CREATE INDEX idx_sender_name ON chat_messages(sender_name)")
        conn.execute("CREATE INDEX idx_message_type ON chat_messages(message_type)")
        
        # Add metadata table
        conn.execute("""
            CREATE TABLE chat_metadata (
                chat_id VARCHAR,
                chat_topic VARCHAR,
                chat_type VARCHAR,
                created_datetime TIMESTAMP,
                last_updated_datetime TIMESTAMP,
                export_datetime TIMESTAMP,
                total_messages INTEGER
            )
        """)
        
        conn.execute("""
            INSERT INTO chat_metadata VALUES (?, ?, ?, ?, ?, ?, ?)
        """, (
            selected_chat.id,
            selected_chat.topic or "No topic",
            selected_chat.chat_type,
            selected_chat.created_date_time,
            selected_chat.last_updated_date_time,
            datetime.now(),
            len(messages)
        ))
        
        conn.close()
        
        print(f'\n✅ Export completed successfully!')
        print(f'📁 Database file: {db_filename}')
        print(f'📊 Total messages exported: {len(messages)}')
        print(f'💾 File size: {os.path.getsize(db_filename) / 1024:.2f} KB')
        
        print('\nSample queries you can run on this database:')
        print('  SELECT sender_name, COUNT(*) as message_count FROM chat_messages GROUP BY sender_name;')
        print('  SELECT DATE(created_datetime) as date, COUNT(*) as messages FROM chat_messages GROUP BY DATE(created_datetime) ORDER BY date;')
        print('  SELECT * FROM chat_messages WHERE content LIKE \'%keyword%\' ORDER BY created_datetime;')
        print()
        
    except ValueError:
        print('Invalid input.\n')
    except Exception as e:
        print(f'Error during export: {e}\n')
# </ExportOneOnOneChatSnippet>

# <MakeGraphCallSnippet>
async def make_graph_call(graph: Graph):
    await graph.make_graph_call()
# </MakeGraphCallSnippet>

# Run main
asyncio.run(main())