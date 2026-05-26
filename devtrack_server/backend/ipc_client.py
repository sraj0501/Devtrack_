"""
IPC Communication Module for Python

This module provides the Python side of IPC communication with the Go daemon.
It handles receiving triggers from Go and sending responses back.
"""

import json
import os
import socket
import time
from datetime import datetime
from enum import Enum
from typing import Callable, Dict, Any, Optional
import logging
from threading import Thread, Lock

# Set up logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class MessageType(str, Enum):
    """IPC Message types"""
    # Go -> Python
    COMMIT_TRIGGER = "commit_trigger"
    TIMER_TRIGGER = "timer_trigger"
    REPORT_TRIGGER = "report_trigger"  # Trigger daily report generation
    STATUS_QUERY = "status_query"
    SHUTDOWN = "shutdown"
    CONFIG_UPDATE = "config_update"

    # Python -> Go
    RESPONSE = "response"
    TASK_UPDATE = "task_update"
    ERROR = "error"
    ACK = "ack"
    PROMPT_REQUEST = "prompt_request"

    # Bidirectional sync & webhook events
    WEBHOOK_EVENT = "webhook_event"
    EXTERNAL_UPDATE = "external_update"

    # Workspace management
    WORKSPACE_RELOAD = "workspace_reload"


class IPCMessage:
    """Represents an IPC message"""

    def __init__(
        self,
        msg_type: MessageType,
        data: Dict[str, Any],
        msg_id: Optional[str] = None,
        error: Optional[str] = None
    ):
        self.type = msg_type
        # Use timezone-aware timestamp (RFC3339 format for Go compatibility)
        from datetime import timezone
        self.timestamp = datetime.now(timezone.utc).astimezone().isoformat()
        self.id = msg_id or f"{msg_type.value}_{int(time.time() * 1000000)}"
        self.data = data
        self.error = error

    def to_dict(self) -> Dict[str, Any]:
        """Convert message to dictionary"""
        result = {
            "type": self.type.value,
            "timestamp": self.timestamp,
            "id": self.id,
            "data": self.data
        }
        if self.error:
            result["error"] = self.error
        return result

    def to_json(self) -> str:
        """Convert message to JSON string"""
        return json.dumps(self.to_dict())

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'IPCMessage':
        """Create message from dictionary"""
        return cls(
            msg_type=MessageType(data["type"]),
            data=data.get("data", {}),
            msg_id=data.get("id"),
            error=data.get("error")
        )

    @classmethod
    def from_json(cls, json_str: str) -> 'IPCMessage':
        """Create message from JSON string"""
        data = json.loads(json_str)
        return cls.from_dict(data)


class IPCClient:
    """Python IPC client for communicating with Go daemon"""

    def __init__(self, socket_path: Optional[str] = None):
        self.socket_path = socket_path or self._get_socket_path()
        self.sock: Optional[socket.socket] = None
        self.connected = False
        self.running = False
        self.lock = Lock()
        self.handlers: Dict[MessageType, Callable] = {}
        self.listener_thread: Optional[Thread] = None

    def _get_socket_path(self) -> str:
        """Get IPC server address (required: IPC_HOST, IPC_PORT from .env)"""
        from backend.config import ipc_host, ipc_port
        return f"{ipc_host()}:{ipc_port()}"

    def connect(self, timeout: int = 5, retry_count: int = 3) -> bool:
        """
        Connect to the IPC server

        Args:
            timeout: Connection timeout in seconds
            retry_count: Number of connection retry attempts

        Returns:
            True if connected successfully
        """
        from backend.config import ipc_retry_delay_ms
        retry_delay_secs = ipc_retry_delay_ms() / 1000.0

        for attempt in range(retry_count):
            try:
                with self.lock:
                    if self.connected:
                        logger.info("Already connected to IPC server")
                        return True

                    # Create TCP socket
                    self.sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
                    self.sock.settimeout(timeout)

                    # Parse address:port
                    host, port = self.socket_path.split(':')
                    self.sock.connect((host, int(port)))
                    self.connected = True

                    logger.info(f"Connected to IPC server at {self.socket_path}")
                    return True

            except FileNotFoundError:
                logger.warning(f"Socket file not found: {self.socket_path}")
                if attempt < retry_count - 1:
                    logger.info(f"Retrying connection (attempt {attempt + 2}/{retry_count})...")
                    time.sleep(retry_delay_secs)
            except ConnectionRefusedError:
                logger.warning("Connection refused by IPC server")
                if attempt < retry_count - 1:
                    logger.info(f"Retrying connection (attempt {attempt + 2}/{retry_count})...")
                    time.sleep(retry_delay_secs)
            except Exception as e:
                logger.error(f"Failed to connect to IPC server: {e}")
                if attempt < retry_count - 1:
                    time.sleep(retry_delay_secs)

        logger.error("Failed to connect to IPC server after all retries")
        return False

    def disconnect(self):
        """Disconnect from the IPC server"""
        with self.lock:
            if not self.connected:
                return

            if self.sock:
                try:
                    self.sock.close()
                except:
                    pass
                self.sock = None

            self.connected = False
            logger.info("Disconnected from IPC server")

    def send_message(self, message: IPCMessage) -> bool:
        """
        Send a message to the Go daemon

        Args:
            message: The message to send

        Returns:
            True if sent successfully
        """
        with self.lock:
            if not self.connected or not self.sock:
                logger.error("Not connected to IPC server")
                return False

            try:
                json_data = message.to_json()
                # Add newline delimiter
                data = (json_data + '\n').encode('utf-8')
                self.sock.sendall(data)
                logger.debug(f"Sent message: {message.type}")
                return True
            except Exception as e:
                logger.error(f"Failed to send message: {e}")
                self.connected = False
                return False

    def receive_message(self, timeout: Optional[float] = None) -> Optional[IPCMessage]:
        """
        Receive a message from the Go daemon

        Args:
            timeout: Receive timeout in seconds (None for blocking)

        Returns:
            The received message or None if error
        """
        with self.lock:
            if not self.connected or not self.sock:
                logger.error("Not connected to IPC server")
                return None

            try:
                if timeout is not None:
                    self.sock.settimeout(timeout)
                else:
                    self.sock.settimeout(None)

                # Read until newline
                data = b''
                while True:
                    chunk = self.sock.recv(1)
                    if not chunk:
                        if data:
                            logger.warning("Connection closed while receiving message")
                        return None
                    if chunk == b'\n':
                        break
                    data += chunk

                if not data:
                    return None

                json_str = data.decode('utf-8')
                message = IPCMessage.from_json(json_str)
                logger.debug(f"Received message: {message.type}")
                return message

            except socket.timeout:
                return None
            except Exception as e:
                logger.error(f"Failed to receive message: {e}")
                self.connected = False
                return None

    def register_handler(self, msg_type: MessageType, handler: Callable[[IPCMessage], None]):
        """
        Register a handler function for a message type

        Args:
            msg_type: The message type to handle
            handler: The handler function
        """
        self.handlers[msg_type] = handler
        logger.info(f"Registered handler for message type: {msg_type}")

    def start_listening(self):
        """Start listening for messages in a background thread"""
        if self.running:
            logger.warning("Already listening for messages")
            return

        self.running = True
        self.listener_thread = Thread(target=self._listen_loop, daemon=True)
        self.listener_thread.start()
        logger.info("Started listening for IPC messages")

    def stop_listening(self):
        """Stop listening for messages"""
        self.running = False
        if self.listener_thread:
            self.listener_thread.join(timeout=2)
        logger.info("Stopped listening for IPC messages")

    def _listen_loop(self):
        """Main listening loop (runs in background thread)"""
        while self.running and self.connected:
            try:
                message = self.receive_message(timeout=1.0)
                if message is None:
                    continue

                # Handle shutdown message
                if message.type == MessageType.SHUTDOWN:
                    logger.info("Received shutdown message")
                    self.running = False
                    break

                # Call registered handler
                handler = self.handlers.get(message.type)
                if handler:
                    try:
                        handler(message)
                    except Exception as e:
                        logger.error(f"Error in handler for {message.type}: {e}")
                        # Send error response
                        error_msg = IPCMessage(
                            msg_type=MessageType.ERROR,
                            msg_id=message.id,
                            data={},
                            error=str(e)
                        )
                        self.send_message(error_msg)
                else:
                    logger.warning(f"No handler registered for message type: {message.type}")

            except Exception as e:
                if self.running:
                    logger.error(f"Error in listen loop: {e}")
                    from backend.config import ipc_retry_delay_ms
                    time.sleep(ipc_retry_delay_ms() / 1000.0)


def create_response_message(request_id: str, data: Dict[str, Any]) -> IPCMessage:
    """Create a response message"""
    return IPCMessage(
        msg_type=MessageType.RESPONSE,
        msg_id=request_id,
        data=data
    )


def create_task_update_message(
    project: str,
    ticket_id: str,
    description: str,
    status: str,
    time_spent: str,
    synced: bool = False,
    azure_work_item_id: Optional[int] = None,
    synced_platform: Optional[str] = None,
) -> IPCMessage:
    """Create a task update message"""
    data = {
        "project": project,
        "ticket_id": ticket_id,
        "description": description,
        "status": status,
        "time_spent": time_spent,
        "synced": synced,
    }
    if azure_work_item_id is not None:
        data["azure_work_item_id"] = azure_work_item_id
    if synced_platform is not None:
        data["synced_platform"] = synced_platform
    return IPCMessage(
        msg_type=MessageType.TASK_UPDATE,
        data=data,
    )


def create_error_message(request_id: str, error: str) -> IPCMessage:
    """Create an error message"""
    return IPCMessage(
        msg_type=MessageType.ERROR,
        msg_id=request_id,
        data={},
        error=error
    )


def create_ack_message(request_id: str) -> IPCMessage:
    """Create an acknowledgment message"""
    return IPCMessage(
        msg_type=MessageType.ACK,
        msg_id=request_id,
        data={"status": "received"}
    )


# Example usage
if __name__ == "__main__":
    # Test IPC client
    client = IPCClient()

    # Define handlers
    def handle_commit_trigger(msg: IPCMessage):
        print(f"Received commit trigger: {msg.data}")
        # Send acknowledgment
        ack = create_ack_message(msg.id)
        client.send_message(ack)

    def handle_timer_trigger(msg: IPCMessage):
        print(f"Received timer trigger: {msg.data}")
        # Send acknowledgment
        ack = create_ack_message(msg.id)
        client.send_message(ack)

    # Register handlers
    client.register_handler(MessageType.COMMIT_TRIGGER, handle_commit_trigger)
    client.register_handler(MessageType.TIMER_TRIGGER, handle_timer_trigger)

    # Connect and listen
    if client.connect():
        print("Connected to IPC server")
        client.start_listening()

        # Keep running
        try:
            while True:
                time.sleep(1)
        except KeyboardInterrupt:
            print("\nShutting down...")
            client.stop_listening()
            client.disconnect()
    else:
        print("Failed to connect to IPC server")
