# kubectl-cron-logs

Kubernetes extension for getting logs of all CronJobs by name.

## Usage
```sh
kubectl cron logs <name> 
```

### Supported Flags
- `-h / --help` Print help info
- `-v / --version` Print current version of plugin
- `-n / --namespace` Specify namespace of cronjob
- `-c / --container` Specify container to get logs from
- `-f / --follow` Follow/stream logs
- `--timestamps` Print timestamps with logs