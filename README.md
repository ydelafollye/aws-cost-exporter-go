# AWS Cost Exporter

A Prometheus exporter for collecting AWS costs via the Cost Explorer API.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                         exporter.Run()                              │
│                               │                                     │
│         ┌─────────────────────┴─────────────────────┐               │
│         │                                           │               │
│         ▼                                           ▼               │
│  ┌─────────────────┐                      ┌─────────────────┐       │
│  │  go server.Start│                      │  go poller.Run  │       │
│  │  (1 goroutine)  │                      │  (1 goroutine)  │       │
│  └────────┬────────┘                      └────────┬────────┘       │
│           │                                        │                │
│           │ Accept()                               │ ticker.C       │
│           ▼                                        ▼                │
│    ┌──────────────┐                         Refresh()               │
│    │ go handler 1 │                              │                  │
│    │ go handler 2 │                    ┌─────────┼─────────┐        │
│    │ go handler 3 │                    │         │         │        │
│    │ ...          │                    ▼         ▼         ▼        │
│    └──────────────┘               go fetch   go fetch   go fetch    │
│    (1 goroutine                   account 1  account 2  account 3   │
│     per request)                  (1 goroutine per AWS account)     │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Components

| Component | Role | Goroutines |
|-----------|------|------------|
| `exporter.Run()` | Orchestrator, keeps the app alive | Spawns server + poller |
| `server.Start()` | HTTP server (endpoints /metrics, /healthz, /readyz) | 1 per HTTP request |
| `poller.Run()` | Periodically refreshes AWS data | 1 (itself) |
| `collector.Refresh()` | Fetches costs from AWS Cost Explorer | 1 per AWS account |

### Data Flow

```
Prometheus ──► GET /metrics ──► Collector.Collect() ──► Cached metrics
                                                              ▲
                                                              │
Poller (ticker) ──► Collector.Refresh() ──► AWS Cost Explorer API
                           │
                           └──► 1 goroutine per AWS account (parallel)
```

### Graceful Shutdown

```
SIGINT/SIGTERM
      │
      ▼
ctx.Done() is triggered
      │
      ├──► Poller stops (return ctx.Err())
      │
      └──► server.Shutdown()
               │
               ├──► Stop Accept() (no new connections)
               ├──► Wait for in-flight requests (10s timeout)
               └──► Return
```

## Configuration

Copy `config.example.yaml` to `config.yaml` and edit it with your AWS account details:

```bash
cp config.example.yaml config.yaml
```

See `config.example.yaml` for a configuration example.
