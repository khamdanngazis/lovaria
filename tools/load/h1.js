// Load test skenario H-1 acara (T17): 200 page view/detik ke undangan +
// 50 RSVP/detik tersebar di banyak tamu. Data dari tools/loadseed.
//
//   k6 run -e BASE_URL=http://localhost:8080 -e DATA=/tmp/lovoria-load.json tools/load/h1.js
//
// Target (Railway): p95 RSVP < 300 ms, p95 halaman publik < 500 ms.
// Rate limit RSVP 6/menit per kode → butuh ≥ 500 kode tamu untuk 50 RSVP/detik.
import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';

const BASE = __ENV.BASE_URL || 'http://localhost:8080';
const DURATION = __ENV.DURATION || '2m';
const data = new SharedArray('data', () => [JSON.parse(open(__ENV.DATA || '/tmp/lovoria-load.json'))]);

export const options = {
  scenarios: {
    pages: {
      executor: 'constant-arrival-rate', exec: 'pageView', rate: 200, timeUnit: '1s',
      duration: DURATION, preAllocatedVUs: 100, maxVUs: 400,
    },
    rsvp: {
      executor: 'constant-arrival-rate', exec: 'rsvp', rate: 50, timeUnit: '1s',
      duration: DURATION, preAllocatedVUs: 50, maxVUs: 200,
    },
  },
  thresholds: {
    'http_req_duration{kind:rsvp_post}': ['p(95)<300'],
    'http_req_duration{kind:page}': ['p(95)<500'],
    'checks': ['rate>0.99'],
  },
};

const pick = (arr) => arr[Math.floor(Math.random() * arr.length)];

// Halaman publik: campuran link umum (/w/slug) dan link tamu (/i/kode).
export function pageView() {
  const d = data[0];
  const path = Math.random() < 0.3 ? `/w/${pick(d.slugs)}` : `/i/${pick(d.codes)}`;
  const res = http.get(BASE + path, { tags: { kind: 'page' } });
  check(res, { 'page 200': (r) => r.status === 200 });
}

// RSVP: buka undangan (ambil token form), lalu kirim jawaban lewat htmx.
export function rsvp() {
  const code = pick(data[0].codes);
  const page = http.get(`${BASE}/i/${code}`, { tags: { kind: 'rsvp_page' } });
  const m = page.body && page.body.match(/action="\/i\/[A-Z0-9]+\/rsvp"[\s\S]*?name="token" value="([^"]+)"/);
  if (!check(m, { 'token RSVP ada': (x) => !!x })) return;
  const res = http.post(`${BASE}/i/${code}/rsvp`,
    { token: m[1], status: Math.random() < 0.8 ? 'attending' : 'declined', pax: '2', message: 'Selamat! 🎉' },
    { headers: { 'HX-Request': 'true' }, tags: { kind: 'rsvp_post' } });
  check(res, { 'rsvp 200': (r) => r.status === 200 });
}
