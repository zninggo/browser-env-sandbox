'use strict';
// 真实 bes-server 冒烟测试:Call 成功路径。
// 前置:bes-server 已在 BES_TEST_ADDR(默认 localhost:18080)监听且无 auth。
//   BES_TEST_ADDR=127.0.0.1:18080 node sdk/node/bes.test.js
//
// 覆盖契约:SDK 发送 {function:...,args:[...]}(见 bes.js:91),
// bridge callRequest.Function(json tag "function")匹配,
// /call 返回 200 且 result 为函数返回值。
const { Sandbox } = require('./bes');

async function probe(addr) {
  try {
    const r = await fetch(`http://${addr}/api/session`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: '{}',
    });
    return r.ok || r.status === 400; // 400 也说明 server 在响应
  } catch { return false; }
}

async function main() {
  const addr = process.env.BES_TEST_ADDR || 'localhost:18080';
  if (!(await probe(addr))) {
    console.log(`SKIP: bes-server not reachable at ${addr}`);
    return;
  }

  const s = new Sandbox(addr, {});
  await s._ready;
  try {
    await s.eval('function add(a,b){return String(Number(a)+Number(b))}');
    const got = await s.call('add', '1', '2');
    const want = '3';
    if (got !== want) throw new Error(`Call result = ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
    console.log(`Call(add,1,2) = ${JSON.stringify(got)} ✓`);
  } finally {
    await s.close();
  }
}

main().catch(e => { console.error('FAIL:', e.message); process.exit(1); });
