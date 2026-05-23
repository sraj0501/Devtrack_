#!/usr/bin/env python3
"""
Teams Chat Database Analyzer
Reads and analyzes exported Teams chat data from DuckDB files
"""

import duckdb
import os
import sys
from datetime import datetime
import glob

# Add project root for imports
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))

class ChatAnalyzer:
    def __init__(self, db_path: str):
        """Initialize the analyzer with a DuckDB file path"""
        if not os.path.exists(db_path):
            raise FileNotFoundError(f"Database file not found: {db_path}")
        
        self.db_path = db_path
        self.conn = duckdb.connect(db_path, read_only=True)
        self._load_metadata()
    
    def _load_metadata(self):
        """Load chat metadata from the database"""
        try:
            result = self.conn.execute("SELECT * FROM chat_metadata").fetchone()
            if result:
                self.metadata = {
                    'chat_id': result[0],
                    'chat_topic': result[1],
                    'chat_type': result[2],
                    'created_datetime': result[3],
                    'last_updated_datetime': result[4],
                    'export_datetime': result[5],
                    'total_messages': result[6]
                }
            else:
                self.metadata = {}
        except Exception:
            self.metadata = {}
    
    def show_summary(self):
        """Display a summary of the chat"""
        print("=" * 60)
        print("TEAMS CHAT ANALYSIS SUMMARY")
        print("=" * 60)
        
        if self.metadata:
            print(f"Chat Topic: {self.metadata.get('chat_topic', 'Unknown')}")
            print(f"Chat Type: {self.metadata.get('chat_type', 'Unknown')}")
            print(f"Created: {self.metadata.get('created_datetime', 'Unknown')}")
            print(f"Last Updated: {self.metadata.get('last_updated_datetime', 'Unknown')}")
            print(f"Export Date: {self.metadata.get('export_datetime', 'Unknown')}")
            print(f"Total Messages: {self.metadata.get('total_messages', 'Unknown')}")
        
        # Get basic statistics
        stats = self.conn.execute("""
            SELECT 
                COUNT(*) as total_messages,
                COUNT(DISTINCT sender_name) as unique_senders,
                MIN(created_datetime) as first_message,
                MAX(created_datetime) as last_message,
                AVG(LENGTH(content)) as avg_message_length
            FROM chat_messages
            WHERE is_deleted = false
        """).fetchone()
        
        if stats:
            print(f"\nMessage Statistics:")
            print(f"Total Messages: {stats[0]}")
            print(f"Unique Senders: {stats[1]}")
            print(f"First Message: {stats[2]}")
            print(f"Last Message: {stats[3]}")
            print(f"Average Message Length: {stats[4]:.1f} characters")
        
        print("=" * 60)
    
    def sender_statistics(self):
        """Show statistics by sender"""
        print("\nSENDER STATISTICS")
        print("-" * 40)
        
        result = self.conn.execute("""
            SELECT 
                sender_name,
                COUNT(*) as message_count,
                AVG(LENGTH(content)) as avg_length,
                MIN(created_datetime) as first_message,
                MAX(created_datetime) as last_message
            FROM chat_messages 
            WHERE is_deleted = false
            GROUP BY sender_name 
            ORDER BY message_count DESC
        """).fetchall()
        
        for row in result:
            print(f"Sender: {row[0]}")
            print(f"  Messages: {row[1]}")
            print(f"  Avg Length: {row[2]:.1f} chars")
            print(f"  First: {row[3]}")
            print(f"  Last: {row[4]}")
            print()
    
    def daily_activity(self, days: int = 30):
        """Show daily message activity for the last N days"""
        print(f"\nDAILY ACTIVITY (Last {days} days)")
        print("-" * 40)
        
        result = self.conn.execute("""
            SELECT 
                DATE(created_datetime) as date,
                COUNT(*) as messages,
                COUNT(DISTINCT sender_name) as active_senders
            FROM chat_messages 
            WHERE is_deleted = false 
                AND created_datetime >= CURRENT_DATE - INTERVAL ? DAY
            GROUP BY DATE(created_datetime) 
            ORDER BY date DESC
        """, [days]).fetchall()
        
        if result:
            for row in result:
                print(f"{row[0]}: {row[1]} messages, {row[2]} senders")
        else:
            print("No activity in the specified period.")
    
    def search_messages(self, keyword: str, limit: int = 10):
        """Search for messages containing a keyword"""
        print(f"\nSEARCH RESULTS for '{keyword}' (showing top {limit})")
        print("-" * 60)
        
        result = self.conn.execute("""
            SELECT 
                sender_name,
                created_datetime,
                content
            FROM chat_messages 
            WHERE LOWER(content) LIKE LOWER(?) 
                AND is_deleted = false
            ORDER BY created_datetime DESC
            LIMIT ?
        """, [f'%{keyword}%', limit]).fetchall()
        
        if result:
            for i, row in enumerate(result, 1):
                timestamp = row[1].strftime('%Y-%m-%d %H:%M:%S') if row[1] else 'Unknown'
                clean_content = self._clean_html_tags(row[2]) if row[2] else ""
                
                if len(clean_content) > 200:
                    clean_content = clean_content[:200] + "..."
                
                print(f"\n{i}. {timestamp} - {row[0]}")
                print(f"   Message: {clean_content}")
        else:
            print(f"No messages found containing '{keyword}'")
    
    def hourly_pattern(self):
        """Show hourly activity pattern"""
        print("\nHOURLY ACTIVITY PATTERN")
        print("-" * 30)
        
        result = self.conn.execute("""
            SELECT 
                EXTRACT(HOUR FROM created_datetime) as hour,
                COUNT(*) as messages
            FROM chat_messages 
            WHERE is_deleted = false
            GROUP BY EXTRACT(HOUR FROM created_datetime)
            ORDER BY hour
        """).fetchall()
        
        if result:
            for row in result:
                hour = int(row[0])
                count = row[1]
                bar = "█" * (count // max(1, max([r[1] for r in result]) // 20))
                print(f"{hour:2d}:00 - {hour+1:2d}:00 │ {count:4d} {bar}")
    
    def export_to_csv(self, output_file: str = None):
        """Export messages to CSV file"""
        if output_file is None:
            timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
            output_file = f"chat_export_{timestamp}.csv"
        
        result = self.conn.execute("""
            SELECT 
                message_id,
                sender_name,
                sender_email,
                created_datetime,
                content,
                message_type,
                is_deleted
            FROM chat_messages 
            ORDER BY created_datetime
        """).fetchdf()
        
        result.to_csv(output_file, index=False)
        print(f"\nMessages exported to: {output_file}")
        print(f"Total records: {len(result)}")
    
    def get_message_timeline(self, sender: str = None):
        """Get chronological timeline of messages"""
        print(f"\nMESSAGE TIMELINE" + (f" for {sender}" if sender else ""))
        print("-" * 50)
        
        query = """
            SELECT 
                sender_name,
                created_datetime,
                content
            FROM chat_messages 
            WHERE is_deleted = false
        """
        params = []
        
        if sender:
            query += " AND LOWER(sender_name) LIKE LOWER(?)"
            params.append(f'%{sender}%')
        
        query += " ORDER BY created_datetime"
        
        result = self.conn.execute(query, params).fetchall()
        
        for row in result:
            timestamp = row[1].strftime('%Y-%m-%d %H:%M:%S') if row[1] else 'Unknown'
            
            # Clean HTML tags from content
            content = self._clean_html_tags(row[2]) if row[2] else ""
            
            # Show complete message wrapped in braces
            print(f"{timestamp} - {row[0]}: {{{content}}}")
    
    def _clean_html_tags(self, text: str) -> str:
        """Remove HTML tags and decode common HTML entities"""
        try:
            from backend.utils.text_utils import clean_html_tags
            return clean_html_tags(text)
        except ImportError:
            import re
            if not text:
                return ""
            text = re.sub(r'<[^>]+>', '', text)
            for entity, replacement in {'&nbsp;': ' ', '&amp;': '&', '&lt;': '<', '&gt;': '>', '&quot;': '"', '&#39;': "'", '&apos;': "'"}.items():
                text = text.replace(entity, replacement)
            return ' '.join(text.split())
        
        return text
    
    def close(self):
        """Close the database connection"""
        self.conn.close()

def find_database_files():
    """Find all Teams chat database files in the current directory"""
    db_files = glob.glob("teams_chat_*.duckdb")
    return db_files

def main():
    """Main interactive interface"""
    print("Teams Chat Database Analyzer")
    print("=" * 40)
    
    # Find available database files
    db_files = find_database_files()
    
    if not db_files:
        print("No Teams chat database files found in current directory.")
        print("Expected files: teams_chat_*.duckdb")
        return
    
    # Select database file
    if len(db_files) == 1:
        db_path = db_files[0]
        print(f"Using database: {db_path}")
    else:
        print("Multiple database files found:")
        for i, file in enumerate(db_files, 1):
            print(f"  {i}. {file}")
        
        try:
            choice = int(input("Select a file (number): ")) - 1
            if 0 <= choice < len(db_files):
                db_path = db_files[choice]
            else:
                print("Invalid selection.")
                return
        except ValueError:
            print("Invalid input.")
            return
    
    # Initialize analyzer
    try:
        analyzer = ChatAnalyzer(db_path)
    except Exception as e:
        print(f"Error opening database: {e}")
        return
    
    # Interactive menu
    while True:
        print("\n" + "=" * 50)
        print("ANALYSIS OPTIONS:")
        print("1. Show summary")
        print("2. Sender statistics")
        print("3. Daily activity")
        print("4. Search messages")
        print("5. Hourly pattern")
        print("6. Export to CSV")
        print("7. Message timeline")
        print("8. Message timeline for specific sender")
        print("0. Exit")
        print("=" * 50)
        
        try:
            choice = input("Select an option (0-8): ").strip()
            
            if choice == '0':
                break
            elif choice == '1':
                analyzer.show_summary()
            elif choice == '2':
                analyzer.sender_statistics()
            elif choice == '3':
                days = input("Number of days (default 30): ").strip()
                days = int(days) if days.isdigit() else 30
                analyzer.daily_activity(days)
            elif choice == '4':
                keyword = input("Enter search keyword: ").strip()
                if keyword:
                    limit = input("Number of results (default 10): ").strip()
                    limit = int(limit) if limit.isdigit() else 10
                    analyzer.search_messages(keyword, limit)
            elif choice == '5':
                analyzer.hourly_pattern()
            elif choice == '6':
                filename = input("Output filename (optional): ").strip()
                analyzer.export_to_csv(filename if filename else None)
            elif choice == '7':
                analyzer.get_message_timeline()
            elif choice == '8':
                sender = input("Enter sender name: ").strip()
                if sender:
                    analyzer.get_message_timeline(sender)
            else:
                print("Invalid option. Please try again.")
                
        except KeyboardInterrupt:
            print("\nExiting...")
            break
        except Exception as e:
            print(f"Error: {e}")
    
    analyzer.close()
    print("Database connection closed. Goodbye!")

if __name__ == "__main__":
    main()