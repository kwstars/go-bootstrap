```bash
#!/bin/bash
TOKEN="abcd1234-abcd-1234-abcd-1234abcd1234"  # ← Replace your SecretID

curl -v \
  -X PUT \
  -H "X-Consul-Token: $TOKEN" \
  -H "Content-Type: application/json" \
  --data @- \
  http://localhost:8500/v1/agent/service/register <<'EOF'
{
  "ID": "web-01",
  "Name": "web",
  "Tags": ["primary", "v1"],
  "Address": "10.0.1.10",
  "Port": 8080,
  "Meta": {
    "version": "1.2.3",
    "region": "us-east-1"
  },
  "Check": {
    "HTTP": "http://10.0.1.10:8080/health",
    "Interval": "15s",
    "Timeout": "5s",
    "DeregisterCriticalServiceAfter": "60m"
  }
}
EOF
```
