"""Expose one stdio test server on an ephemeral IPv4 loopback socket."""

import os
import socket
import sys

with socket.create_server(("127.0.0.1", 0)) as listener:
    print(f"Listening on port {listener.getsockname()[1]}", flush=True)
    connection, _ = listener.accept()

with connection:
    os.dup2(connection.fileno(), 0)
    os.dup2(connection.fileno(), 1)

os.execvp(sys.argv[1], sys.argv[1:])
