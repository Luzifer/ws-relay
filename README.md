![](https://img.shields.io/github/license/Luzifer/ws-relay)
![](https://img.shields.io/github/downloads/Luzifer/ws-relay/total)
![](https://img.shields.io/github/v/release/Luzifer/ws-relay)

# Luzifer / ws-relay

This project is a very simple WebSocket relay service: No auth, no message parsing, just 1-n clients connecting to the same socket name receiving all messages sent to the socket.

Clients connect to `/{socket}`. Every message received from one client on that socket name is forwarded to all currently connected clients on the same socket name.

## Configuration

- `--listen`: Port/IP to listen on, defaults to `:3000`
- `--log-level`: Log level, defaults to `info`
- `--max-conns-per-socket`: Maximum accepted WebSocket connections per socket name, defaults to `100`; set to `0` to disable the limit
- `--max-message-size-bytes`: Maximum accepted WebSocket message size, defaults to `1048576` (1MiB); set to `0` to disable the limit
- `--version`: Print the current version and exit

## Security

`ws-relay` is intentionally unauthenticated and accepts WebSocket connections from any origin. Socket names are therefore public channel names, not secrets or access controls. Any client that can reach the service can connect to a socket name and send messages to every other client on that same name.

Do not expose this service to untrusted networks for private or integrity-sensitive traffic unless access is controlled elsewhere, for example with a reverse proxy, firewall rules, VPN, or another authentication layer. Keep `--max-conns-per-socket` and `--max-message-size-bytes` enabled for internet-facing deployments to limit resource pressure from excessive connections and oversized messages; disabling either with `0` should be reserved for trusted environments with another limit in front of the service.
