import http from 'k6/http'
import { check } from 'k6'

// 用法：
//   BASE=http://localhost:8080/v1 DEVICE_TOKEN=... k6 run deploy/k6/load.js
//   TARGET=heartbeat GRANT_ID=... k6 run deploy/k6/load.js
//
// 说明：答题/会话接口有设备级限流，压测前应在预发环境关闭限流，
// 或使用多个设备令牌分摊。默认压测 /child/context。

const BASE = __ENV.BASE || 'http://localhost:8080/v1'
const TOKEN = __ENV.DEVICE_TOKEN || ''
const TARGET = __ENV.TARGET || 'context'
const RATE = Number(__ENV.RATE || 1000)
const DURATION = __ENV.DURATION || '30s'

export const options = {
  scenarios: {
    load: {
      executor: 'constant-arrival-rate',
      rate: RATE,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: Math.max(50, Math.ceil(RATE / 50)),
      maxVUs: Math.max(200, RATE),
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<200'],
    http_req_failed: ['rate<0.01'],
  },
}

const headers = {
  Authorization: `Bearer ${TOKEN}`,
  'Content-Type': 'application/json',
}

export default function () {
  if (!TOKEN) {
    throw new Error('DEVICE_TOKEN is required')
  }
  let res
  if (TARGET === 'heartbeat') {
    const grantID = __ENV.GRANT_ID
    if (!grantID) {
      throw new Error('GRANT_ID is required for heartbeat target')
    }
    res = http.post(`${BASE}/watch/grants/${grantID}/heartbeat`,
      JSON.stringify({ foreground_app: 'com.example.video', paused: false }), { headers })
  } else {
    res = http.get(`${BASE}/child/context`, { headers })
  }
  check(res, { 'status 200': (r) => r.status === 200 })
}
