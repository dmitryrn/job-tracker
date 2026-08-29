# Local Observability

Grafana and Loki run as rootless Podman containers owned by `clanker`. Alloy
runs as a native user service because the systemd journal reader needs the
host's supplementary journal group. Container images use fixed digests so
updates are deliberate.

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
systemctl --user start grafana.service alloy.service
```

Create the Grafana admin-password secret before starting Grafana:

```bash
printf '%s' 'replace-with-a-strong-password' | podman secret create grafana_admin_password -
```

Grafana listens on `http://10.8.0.4:3000`. Sign-up and anonymous access are
disabled. Loki only listens on loopback; Alloy sends it journal entries from
`jobs.service`. The journal directory includes this host's machine ID; update
`observability/alloy.alloy` if the machine ID changes.

To check the stack:

```bash
systemctl --user status grafana loki alloy
podman ps
```

In Grafana Explore, select the provisioned Loki data source and query:

```logql
{service="jobs"}
```
