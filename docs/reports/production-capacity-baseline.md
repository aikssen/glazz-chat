# Production Demo Capacity Baseline

- **Measured:** 2026-07-26
- **Scope:** single-node, controlled public demo
- **Result:** a 1 vCPU / 4 GiB host is sufficient; 2 vCPU / 4 GiB is recommended
- **Status:** planning evidence, not production approval or a hosting decision

## Purpose

This report translates the current five-service Glazz topology into an initial
machine size for a small public demo. It records measured runtime behavior rather
than extrapolating from framework defaults.

The target is deliberately modest:

- one instance of each application service;
- tens of active users, not unbounded public traffic;
- bounded LLM concurrency and spend;
- no availability claim or horizontal scaling requirement.

This report informs `PROD-001` and `PROD-010`. It does not close either task.

## Runtime topology

The measured stack contained:

1. the Next.js standalone web server;
2. the Go HTTP and WebSocket API;
3. the Go background worker;
4. PostgreSQL;
5. Redis.

The API and worker use separate processes. PostgreSQL remains authoritative.
Redis holds only ephemeral coordination state such as tickets, leases, rate
limits, replay data, and Pub/Sub.

## Method

The benchmark ran on an x86-64 Linux Docker host using Docker 29.6.2 and Docker
Compose 5.3.1. The host had more capacity than the target, so the isolated stack
was restricted to:

- two shared CPU cores for the 2 vCPU profile;
- one shared CPU core for the 1 vCPU profile.

The benchmark used:

- a separate Compose project, network, and PostgreSQL volume;
- loopback-only ports;
- deterministic test OAuth configuration;
- the six-token fake LLM provider;
- fresh guest sessions and test-only application data.

The complete chat path included guest creation, CSRF, conversation persistence,
WebSocket ticket issuance, authenticated upgrade, generation, quota accounting,
message persistence, and the terminal realtime event.

The benchmark project and volume were removed afterward. The persistent
development stack was not used for generated test data.

## Important limitations

The measurements exclude:

- TLS and a production reverse proxy;
- Internet and cross-region latency;
- a production LLM provider;
- long prompts and responses;
- concurrent backups, deploys, and operating-system updates;
- shared-VPS neighbor contention;
- production telemetry exporters.

Loopback throughput is therefore an infrastructure ceiling, not a production
SLO. LLM generation results validate the application path only; they do not
predict external provider latency, capacity, or cost.

## Container image footprint

### Compressed transfer size

Each image was exported and compressed independently.

| Image                          |   Compressed |
| ------------------------------ | -----------: |
| Web                            |      59.8 MB |
| API                            |      21.5 MB |
| Worker                         |      21.5 MB |
| PostgreSQL                     |     119.2 MB |
| Redis                          |      37.5 MB |
| **Independent-download total** | **259.5 MB** |

### Unpacked virtual size

| Image      | Virtual size |
| ---------- | -----------: |
| Web        |       254 MB |
| API        |      83.4 MB |
| Worker     |      83.4 MB |
| PostgreSQL |       433 MB |
| Redis      |       155 MB |

API and worker share almost all layers. Redis also shares a small Alpine layer
with the Go runtime images. The unique unpacked runtime set is approximately
0.9 GB.

The running containers added less than 50 KB of writable-layer data. Durable
application growth belongs in the PostgreSQL volume, not container layers.

## Memory profile

| Service     | Clean idle |                    Highest observed | Peak scenario                         |
| ----------- | ---------: | ----------------------------------: | ------------------------------------- |
| Next.js web |  34–37 MiB |                             266 MiB | 100 concurrent web requests on 2 CPU  |
| Go API      |    3–9 MiB |                             135 MiB | 1,000 simultaneous WebSockets         |
| Go worker   |    3–5 MiB |                               5 MiB | normal operation during the benchmark |
| PostgreSQL  |  32–75 MiB | about 96 MiB including cgroup cache | sessions and generation persistence   |
| Redis       |    5–6 MiB |                            21.3 MiB | realtime tickets and 1,000 WebSockets |

The clean five-service stack used approximately 77–91 MiB as reported by Docker.
A conservative sum of independent peaks is approximately 500–600 MiB. Those
peaks did not all occur simultaneously.

### Initial container reservations

| Service             | Recommended reservation or limit |
| ------------------- | -------------------------------: |
| Web                 |                          512 MiB |
| API                 |                          256 MiB |
| Worker              |                          128 MiB |
| PostgreSQL          |                      512–768 MiB |
| Redis               |                          128 MiB |
| Reverse proxy       |                           64 MiB |
| **Container total** |            **about 1.6–1.9 GiB** |

The remaining memory on a 4 GiB host is reserved for Linux, Docker, filesystem
cache, administrative tasks, and burst headroom. A small swap file can protect
against short spikes, but it is not a substitute for RAM.

## HTTP capacity

### One-CPU profile

| Workload                                    |        Throughput |           p95 | Errors |
| ------------------------------------------- | ----------------: | ------------: | -----: |
| Web root, 50 concurrent clients             |       930.5 req/s |         82 ms |      0 |
| Guest allowance path, 50 concurrent clients |       6,663 req/s |         11 ms |      0 |
| Mixed: 20 web and 20 API clients            | 403 + 4,204 req/s | 114 ms / 9 ms |      0 |

The guest path exercised the API, PostgreSQL, and Redis. The mixed workload used
approximately one complete CPU while remaining error-free.

### Two-CPU profile

| Workload                                    |   Throughput |   p95 | Errors |
| ------------------------------------------- | -----------: | ----: | -----: |
| Web root, 100 concurrent clients            |  2,257 req/s | 64 ms |      0 |
| Guest allowance path, 50 concurrent clients | 13,763 req/s |  7 ms |      0 |

These request rates are saturation evidence, not a recommended traffic target.
Real browsers cache static assets and do not continuously request the application
root or allowance endpoint.

## WebSocket capacity

The real container path included the guest cookie, CSRF token, single-use ticket,
Redis state, authenticated WebSocket upgrade, and `connection.ready`.

All services shared one CPU.

| Simultaneous connections | Established | Handshake p95 |    API memory | Redis memory | Errors |
| -----------------------: | ----------: | ------------: | ------------: | -----------: | -----: |
|                      500 |         500 |        653 ms |  about 65 MiB | about 12 MiB |      0 |
|                    1,000 |       1,000 |        1.16 s | about 135 MiB | about 18 MiB |      0 |

The observed incremental footprint was approximately 100–120 KiB per open
connection across the API and Redis in this idle-connection profile.

Active streaming, replay retention, slow clients, and larger event payloads can
increase that value. The initial public demo should use a much lower operational
limit.

## Generation-path capacity

The benchmark sent concurrent `chat.generate` commands through the complete
application path using the fake provider.

All services shared one CPU.

| Concurrent generations | Completed |          p95 | Errors |
| ---------------------: | --------: | -----------: | -----: |
|                      2 |         2 | about 336 ms |      0 |
|                     20 |        20 |       505 ms |      0 |
|                    100 |       100 |       872 ms |      0 |

The fake provider emitted six tokens without network delay. A real provider keeps
connections and generation state alive longer, introduces first-token and stream
latency, and creates billable usage. Production concurrency must therefore be
selected from provider capacity and spend limits, not this technical ceiling.

## Data growth

The isolated run created:

- 1,748 guest sessions;
- 122 conversations;
- 244 messages;
- 122 generations;
- the associated quota, usage-ledger, and reservation records.

| Measure                    |        Result |
| -------------------------- | ------------: |
| Initial logical database   |  about 9.1 MB |
| Final logical database     | about 11.2 MB |
| Final PostgreSQL directory |   about 69 MB |
| Final Redis logical memory |       4.44 MB |
| Redis peak logical memory  |      21.29 MB |

The physical PostgreSQL directory includes WAL and minimum internal allocation.
The benchmark messages were intentionally short. Capacity planning should use a
more conservative allowance of 50–100 KB per stored conversation, including
messages, generations, indexes, quota records, and WAL effects.

| Stored conversations | Conservative allowance |
| -------------------: | ---------------------: |
|                1,000 |              50–100 MB |
|               10,000 |               0.5–1 GB |
|               50,000 |               2.5–5 GB |

The MVP has no file or image uploads, so it does not need local object-storage
capacity.

## Disk budget

The production host must not build application images. CI should produce immutable
images and publish them to a registry; the host should only pull reviewed
artifacts. Development build cache was substantially larger than the complete
runtime and is not part of the production budget.

| Purpose                                | Initial allowance |
| -------------------------------------- | ----------------: |
| Linux, Docker, and system updates      |           6–10 GB |
| Current images and rollback artifacts  |            2–3 GB |
| PostgreSQL data                        |              5 GB |
| Rotated local logs                     |        0.2–0.5 GB |
| Temporary files and operating headroom |           5–10 GB |
| **Practical minimum**                  |         **20 GB** |
| **Recommended disk**                   |         **40 GB** |

Docker logging must be bounded. A suitable starting point is:

```yaml
logging:
  driver: json-file
  options:
    max-size: "10m"
    max-file: "3"
```

Backups must be encrypted and stored outside the host. Host-level snapshots do
not replace a tested PostgreSQL restore procedure.

## Recommended demo machine

### Minimum technical profile

- 1 vCPU;
- 2 GiB RAM;
- 20 GB disk;
- 1–2 GiB swap;
- externally built images.

This profile can run Glazz but leaves limited margin for backups, deploys,
updates, and combined bursts.

### Recommended profile

- 2 vCPU preferred; 1 vCPU remains acceptable;
- 4 GiB RAM;
- 40–50 GB NVMe disk;
- 1–2 GiB swap;
- one replica per service;
- externally built, immutable images;
- off-host PostgreSQL backups;
- bounded Redis and log memory;
- explicit per-container resource limits.

## Conservative launch envelope

The first public demo should stay well below the measured ceiling:

- 20–50 initially admitted users;
- 25–50 simultaneously active browsers;
- 100–200 open WebSockets;
- 5–10 concurrent LLM generations globally;
- one concurrent generation per user;
- daily message and output-token quotas;
- a hard provider-spend limit.

The LLM provider, not machine CPU, is expected to be the first cost boundary.

## Revalidation required before launch

Repeat the capacity test after production infrastructure is selected, using:

1. the exact immutable release images;
2. the production reverse proxy and TLS path;
3. production-like PostgreSQL and Redis settings;
4. the approved LLM provider and model;
5. realistic prompt, response, and conversation sizes;
6. backup and telemetry processes running;
7. the purchased CPU, memory, disk, network, and region;
8. the final generation and spend limits.

The resulting evidence should define alert thresholds, the controlled-launch
limit, and the rollback trigger for `PROD-010`.
