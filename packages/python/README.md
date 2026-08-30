# Hostero Python SDK

The Python SDK for the [Hostero API](https://api.hostero.gg/v1/openapi.json).

> This package is currently in beta.

## Install

```bash
python -m pip install --pre hostero
```

## Use

```python
from hostero import Hostero

with Hostero.from_env() as client:
    for server in client.servers.list().items:
        print(server.name)
```

`Hostero.from_env()` reads `HOSTERO_API_KEY`. For explicit configuration:

```python
client = Hostero(api_key="hst_...", base_url="https://api.hostero.gg/v1")
```

See the [repository](https://github.com/hoxger/hostero-sdk) for development and
DevKit details.
