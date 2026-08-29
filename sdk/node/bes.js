// bes Node.js SDK — JSON-RPC over HTTP client
// No gRPC dependency needed.
//
// 用法一（推荐）：静态工厂，返回已就绪实例
//   const sandbox = await Sandbox.create({ serverAddr: 'localhost:19821' });
//   await sandbox.eval('navigator.userAgent');
//
// 用法二：构造后 await ready
//   const sandbox = new Sandbox();
//   await sandbox.ready;
//   await sandbox.eval('navigator.userAgent');

class Sandbox {
  // Async factory: resolves once the session is created and usable.
  static async create(opts = {}) {
    const { serverAddr = 'localhost:19821', ...rest } = opts;
    const s = new Sandbox(serverAddr, rest);
    await s._ready;
    return s;
  }

  constructor(serverAddr = 'localhost:19821', opts = {}) {
    this.baseURL = `http://${serverAddr}`;
    this.sessionId = null;
    // D5 fix: the constructor fires session creation async; every API awaits
    // this promise first so `new Sandbox(...).eval(...)` no longer races
    // "Session not created".
    this._ready = this._createSession(opts);
  }

  async _post(path, data) {
    const resp = await fetch(`${this.baseURL}${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!resp.ok) {
      const text = await resp.text();
      throw new Error(`HTTP ${resp.status}: ${text}`);
    }
    return resp.json();
  }

  async _get(path) {
    const resp = await fetch(`${this.baseURL}${path}`);
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    return resp.json();
  }

  async _delete(path) {
    const resp = await fetch(`${this.baseURL}${path}`, { method: 'DELETE' });
    return resp.json();
  }

  async _createSession(opts) {
    const { browser = 'chrome', os = 'windows', seed = 0, location = 'https://example.com/',
            cookies = {}, proxy = '', netMode = 'live', recording = '' } = opts;
    const resp = await this._post('/api/session', {
      browser, os, seed, location, cookies, proxy, net_mode: netMode, recording,
    });
    if (resp.error) throw new Error(resp.error);
    this.sessionId = resp.session_id;
    return this;
  }

  async _ensureSession() {
    await this._ready;
    if (!this.sessionId) throw new Error('Session not created');
  }

  async eval(code) {
    await this._ensureSession();
    const resp = await this._post(`/api/session/${this.sessionId}/eval`, { code });
    if (resp.error) throw new Error(resp.error);
    return resp.result || '';
  }

  async loadScript(name, content) {
    await this._ensureSession();
    if (content === undefined) {
      const fs = require('fs');
      content = fs.readFileSync(name, 'utf-8');
    }
    const resp = await this._post(`/api/session/${this.sessionId}/script`, { name, content });
    if (resp.error) throw new Error(resp.error);
  }

  async call(functionName, ...args) {
    await this._ensureSession();
    const resp = await this._post(`/api/session/${this.sessionId}/call`, {
      function_name: functionName, args,
    });
    if (resp.error) throw new Error(resp.error);
    return resp.result || '';
  }

  async getFingerprint() {
    await this._ensureSession();
    return this._get(`/api/session/${this.sessionId}/fingerprint`);
  }

  async getCookies() {
    await this._ensureSession();
    const resp = await this._get(`/api/session/${this.sessionId}/cookies`);
    return resp.cookies || '';
  }

  async setCookie(name, value) {
    await this._ensureSession();
    await this._post(`/api/session/${this.sessionId}/cookies`, { name, value });
  }

  async close() {
    await this._ready.catch(() => {});
    if (this.sessionId) {
      try { await this._delete(`/api/session/${this.sessionId}`); } catch (e) {}
      this.sessionId = null;
    }
  }
}

module.exports = { Sandbox };
