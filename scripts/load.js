import http from 'k6/http'
import { check } from 'k6'

export const options = {
  stages: [
    { duration: '15s', target: 80 },
    { duration: '4m', target: 250 },
    { duration: '15s', target: 0 },
  ],
  thresholds: { http_req_failed: ['rate<0.05'] },
}

const base = __ENV.BASE_URL || 'http://backend.school.svc.cluster.local:8080'

export default function () {
  const r = http.get(`${base}/api/events`)
  check(r, { 'HTTP 200': (res) => res.status === 200 })
}
