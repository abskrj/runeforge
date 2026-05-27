# Runeforge

## MCP Server (Docker Compose)

Start just the MCP server (and its dependencies):

```bash
docker compose up -d mcp-server
```

Call the MCP JSON-RPC endpoint:

```bash
curl -s http://localhost:8090/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/list",
    "params": {}
  }'
```

Inside Compose, the MCP server uses `CONTROL_PLANE_URL=http://control-plane:8080`.

## Embed Dashboard (Docker Compose)

Start the embed dashboard service:

```bash
docker compose up -d embed-dashboard
```

Local URL:

```text
http://localhost:8091
```

Create an embed token (manage scope API key required):

```bash
curl -s http://localhost:8080/v1/embed/tokens \
  -H "Authorization: Bearer rf_your_manage_key" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant_slug": "your-tenant-slug",
    "ttl_seconds": 3600
  }'
```

Open the dashboard:

```text
http://localhost:8091/?token=et_your_token
```
