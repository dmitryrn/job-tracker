# Local Observability

Grafana, Loki, and Prometheus run as rootless Podman containers owned by
`clanker`. Alloy runs as a native user service because the systemd journal
reader needs the host's supplementary journal group. Container images use
fixed digests so updates are deliberate.

Before starting Alloy, add `clanker` to the journal-reader groups:

```bash
sudo usermod -aG adm,systemd-journal clanker
```

Install the Quadlet files into the user configuration directory, reload the user
manager, and start the stack:

```bash
mkdir -p ~/.config/containers/systemd
ln -sfn /home/clanker/projects/nice/observability/quadlets/* ~/.config/containers/systemd/
ln -sfn /home/clanker/projects/nice/systemd/alloy.service ~/.config/systemd/user/alloy.service
systemctl --user daemon-reload
systemctl --user start grafana.service prometheus.service alloy.service
```

Create the Grafana admin-password secret before starting Grafana:

```bash
printf '%s' 'replace-with-a-strong-password' | podman secret create grafana_admin_password -
```

Grafana listens on `http://10.8.0.4:3000`. Sign-up and anonymous access are
disabled. Loki only listens on loopback; Prometheus is accessible only to
containers on the isolated observability network. Alloy sends journal entries
from `jobs.service` to Loki. The journal directory includes this host's machine
ID; update `observability/alloy.alloy` if the machine ID changes.

To check the stack:

```bash
systemctl --user status grafana loki prometheus alloy
podman ps
```

In Grafana Explore, select the provisioned Loki data source and query:

```logql
{service="jobs"}
```

## LinkedIn Metrics Plan

Prometheus will collect only application-specific LinkedIn metrics. It will not
collect host CPU, memory, disk, Go runtime, process, or HTTP metrics.

The first Grafana dashboard should be a bar chart per completed LinkedIn run.
Each run should show the job counts that explain its outcome:

- skipped: listings already stored locally
- added: validated listings saved to the jobs database
- invalid: listings rejected because required search or detail data was missing
- errors: search, job-detail, persistence, or IP-gate failures

The dashboard should also show the configured search query for each run and
the run's start and finish times. Query text is display metadata, not a
Prometheus label: it is unbounded and would create high-cardinality time
series. The existing SQLite application events remain the source for that
per-run detail; metrics provide the aggregate counts and timestamps.

The metrics design will use one short-lived gauge per run outcome/count and a
last-success timestamp. Prometheus counters are not suitable for a per-run bar
chart because they accumulate across all runs. The exact names and labels will
be added with the instrumentation after Prometheus is running.
