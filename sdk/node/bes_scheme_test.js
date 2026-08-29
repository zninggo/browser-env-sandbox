// Smoke test for bes Node SDK scheme resolution.
// Zero-dependency: runnable with plain `node sdk/node/bes_scheme_test.js`.
// Verifies the baseURL contract — HTTPS addresses reach the wire as https://,
// bare host:port stays http:// (backward compatibility).

const assert = require('assert');
const { Sandbox, normalizeBaseURL } = require('./bes');

let passed = 0;
function check(name, fn) {
  fn();
  passed++;
  console.log(`  ok - ${name}`);
}

console.log('bes Node SDK scheme smoke test');

check('https:// prefix preserved', () => {
  assert.strictEqual(normalizeBaseURL('https://example.com'), 'https://example.com');
});

check('https:// with port preserved', () => {
  assert.strictEqual(normalizeBaseURL('https://bes.example.com:443'), 'https://bes.example.com:443');
});

check('http:// prefix preserved', () => {
  assert.strictEqual(normalizeBaseURL('http://192.168.1.10:19821'), 'http://192.168.1.10:19821');
});

check('bare host:port defaults to http://', () => {
  assert.strictEqual(normalizeBaseURL('localhost:19821'), 'http://localhost:19821');
});

check('bare host defaults to http://', () => {
  assert.strictEqual(normalizeBaseURL('bes.example.com'), 'http://bes.example.com');
});

// Constructor wires the resolved scheme through to baseURL. We construct
// without awaiting _ready (no real server) — baseURL is set synchronously
// before the session-creation promise fires, so this is a safe smoke check.
// The constructor fires _createSession asynchronously; swallow its inevitable
// rejection so it doesn't surface as an unhandled rejection and skew the exit
// code (a real caller would await Sandbox.create / s._ready and handle errors).
function assertBaseURL(addr, want) {
  const s = new Sandbox(addr);
  s._ready.catch(() => {});
  assert.strictEqual(s.baseURL, want);
}

check('constructor baseURL for https address', () => {
  assertBaseURL('https://example.com', 'https://example.com');
});

check('constructor baseURL for bare address (backward compat)', () => {
  assertBaseURL('localhost:19821', 'http://localhost:19821');
});

console.log(`\nAll ${passed} checks passed.`);
