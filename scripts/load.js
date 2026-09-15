import http from 'k6/http'
import { check } from 'k6'
export const options = { stages: [{ duration: '30s', target: 20 }, { duration: '3m', target: 100 }, { duration: '30s', target: 0 }], thresholds: { http_req_failed: ['rate<0.01'] } }
export default function () { const r = http.get(`${__ENV.BASE_URL || 'http://frontend.school.svc.cluster.local'}/api/events`); check(r, { 'HTTP 200': r => r.status === 200 }) }
