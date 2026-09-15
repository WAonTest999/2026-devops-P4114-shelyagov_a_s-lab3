"""Regenerate the provisioned Grafana dashboard without additional libraries."""
import json
from pathlib import Path

queries = [
    ('HTTP requests by pod, route and method', 'sum by (pod, route, method) (rate(school_http_requests_total{namespace="school"}[1m]))', '{{pod}} {{method}} {{route}}', 'reqps'),
    ('Response status by pod', 'sum by (pod, status) (rate(school_http_requests_total{namespace="school"}[1m]))', '{{pod}} HTTP {{status}}', 'reqps'),
    ('Request latency p95', 'histogram_quantile(0.95, sum by (le, pod, route) (rate(school_http_request_duration_seconds_bucket{namespace="school"}[5m])))', '{{pod}} {{route}}', 's'),
    ('Backend CPU usage (cores)', 'sum by (pod) (rate(container_cpu_usage_seconds_total{namespace="school",container="backend"}[1m]))', '{{pod}}', 'short'),
    ('Backend memory (bytes)', 'sum by (pod) (container_memory_working_set_bytes{namespace="school",container="backend"})', '{{pod}}', 'bytes'),
    ('Available backend replicas', 'kube_deployment_status_replicas_available{namespace="school",deployment="backend"}', 'Available', 'short'),
    ('HPA desired replicas', 'kube_horizontalpodautoscaler_status_desired_replicas{namespace="school",horizontalpodautoscaler="backend"}', 'Desired', 'short'),
    ('Node CPU usage', '1 - avg by (instance) (rate(node_cpu_seconds_total{mode="idle"}[5m]))', '{{instance}}', 'percentunit'),
]
panels = []
for i, (title, expr, legend, unit) in enumerate(queries):
    panels.append({'id': i + 1, 'title': title, 'type': 'timeseries', 'datasource': {'type': 'prometheus', 'uid': '${DS_PROMETHEUS}'}, 'gridPos': {'x': (i % 2) * 12, 'y': (i // 2) * 8, 'w': 12, 'h': 8}, 'targets': [{'refId': 'A', 'expr': expr, 'legendFormat': legend}], 'fieldConfig': {'defaults': {'unit': unit}, 'overrides': []}})
dashboard = {'uid': 'music-school', 'title': 'Music School · Application and Infrastructure', 'schemaVersion': 39, 'version': 1, 'refresh': '10s', 'time': {'from': 'now-15m', 'to': 'now'}, 'templating': {'list': [{'name': 'DS_PROMETHEUS', 'type': 'datasource', 'query': 'prometheus', 'current': {'text': 'Prometheus', 'value': 'prometheus'}}]}, 'panels': panels}
header = 'apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: school-dashboard\n  namespace: monitoring\n  labels: {grafana_dashboard: "1"}\ndata:\n  school.json: |\n'
Path('deploy/monitoring/dashboard.yaml').write_text(header + '\n'.join('    ' + line for line in json.dumps(dashboard, indent=2).splitlines()) + '\n', encoding='utf-8')
