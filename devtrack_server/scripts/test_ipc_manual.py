#!/usr/bin/env python3
"""
DevTrack Phase 0.4: Manual IPC connectivity test.

Connects to the IPC server (requires daemon running) and verifies the connection.
Run: devtrack start (in another terminal) before running this script.

Usage:
    uv run python scripts/test_ipc_manual.py
    # Or with project root in path:
    python scripts/test_ipc_manual.py
"""
import json
import os
import socket
import sys
from pathlib import Path

# Add project root for backend imports
project_root = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(project_root))

def get_ipc_address():
    """Get IPC host:port from config or env."""
    try:
        from backend.config import _load_env, ipc_host, ipc_port
        _load_env()
        return f"{ipc_host()}:{ipc_port()}"
    except ImportError:
        host = os.getenv("IPC_HOST", "127.0.0.1")
        port = os.getenv("IPC_PORT", "35893")
        return f"{host}:{port}"


def main():
    print("==========================================")
    print("Test: IPC Connectivity")
    print("==========================================")
    print("Prerequisite: Run 'devtrack start' in another terminal")
    print("")

    addr = get_ipc_address()
    host, port = addr.split(":")
    port = int(port)

    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(5)
        sock.connect((host, port))
        print(f"  OK: Connected to IPC server at {addr}")
        sock.settimeout(2)
        # Try to receive (server may send STATUS_QUERY when status is run)
        try:
            data = b""
            while True:
                chunk = sock.recv(1)
                if not chunk:
                    break
                data += chunk
                if data.endswith(b"\n"):
                    break
            if data:
                msg = json.loads(data.decode().strip())
                print(f"  OK: Received message type: {msg.get('type', 'unknown')}")
        except socket.timeout:
            print("  OK: No message received (timeout) - server is responsive")
        except json.JSONDecodeError:
            print("  OK: Received data (not JSON) - connection works")
        sock.close()
        print("")
        print("IPC connectivity test passed.")
        return 0
    except ConnectionRefusedError:
        print("  FAIL: Connection refused. Is the daemon running? (devtrack start)")
        return 1
    except Exception as e:
        print(f"  FAIL: {e}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
