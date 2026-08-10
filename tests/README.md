# Test Notes

The current validation path is code-level and protocol-level:

```bash
cd vsp-server && go test ./...
cd vsp-client && go test ./...
cd vsp-windows && go test ./...
cd vsp-windows/frontend && npm run build
cd tests/e2e && go test ./...
```

Legacy WebSocket and com0com end-to-end scripts were removed because the first release only exposes a local TCP gateway. Protocol coverage for session pairing and binary forwarding lives in `vsp-server/internal/relay`, while Windows gateway coverage lives in `vsp-windows/internal/service`.

`tests/e2e` contains a Linux-only pseudo-terminal test that starts a real `vsp-server`, a real `device-agent`, and a real `desktop-gateway`. It creates a temporary user and device, sends bytes from local TCP to the simulated serial port, writes a serial reply, and verifies the reply returns to TCP.
