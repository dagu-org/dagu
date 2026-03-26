# Issue 1837 Reproduction

This stack mirrors the topology reported in [dagu-org/dagu#1837](https://github.com/dagu-org/dagu/issues/1837):

- 1 UI/API service on `http://localhost:8888`
- 3 schedulers
- 3 coordinators
- 2 workers
- shared bind-mounted `data/`, `logs/`, and `dags/` for the UI/scheduler/coordinator tier
- shared-nothing workers using static coordinator discovery (`worker.coordinators`)
- `default_execution_mode: distributed` so scheduled DAGs are dispatched through the coordinator
- explicit global queue config loaded from [`config.yaml`](./config.yaml)

Run it from the repository root:

```bash
docker compose -f deploy/docker/compose.issue-1837.yaml up -d
```

Login for the web UI:

- username: `admin`
- password: `debug-password`

Seeded DAGs:

- `dags/issue-1837-queue-stuck-01.yaml` through `dags/issue-1837-queue-stuck-10.yaml`
- all 10 DAGs are assigned to the named queue `issue-1837`
- all 10 run every minute
- each DAG includes a `sleep 15` in the middle step so the queue builds pressure instead of draining instantly

Useful paths on the host:

- runtime state: `deploy/docker/issue-1837/data`
- logs: `deploy/docker/issue-1837/logs`

To test a locally built image instead of the released `2.3.4` image:

```bash
docker build -t dagu-issue-1837:local .
DAGU_IMAGE=dagu-issue-1837:local docker compose -f deploy/docker/compose.issue-1837.yaml up -d
```
