// bes Node.js SDK — JSON-RPC over HTTP client
// No gRPC dependency needed.

class Sandbox {
  constructor(serverAddr = 'localhost:19821', opts = {}) {
    this.baseURL = `http://${serverAddr}`;
    this.sessionId = null;
    this._createSession(opts);
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

  async eval(code) {
    if (!this.sessionId) throw new Error('Session not created');
    const resp = await this._post(`/api/session/${this.sessionId}/eval`, { code });
    if (resp.error) throw new Error(resp.error);
    return resp.result || '';
  }

  async loadScript(name, content) {
    if (!this.sessionId) throw new Error('Session not created');
    if (content === undefined) {
      const fs = require('fs');
      content = fs.readFileSync(name, 'utf-8');
    }
    const resp = await this._post(`/api/session/${this.sessionId}/script`, { name, content });
    if (resp.error) throw new Error(resp.error);
  }

  async call(functionName, ...args) {
    if (!this.sessionId) throw new Error('Session not created');
    const resp = await this._post(`/api/session/${this.sessionId}/call`, {
      function_name: functionName, args,
    });
    if (resp.error) throw new Error(resp.error);
    return resp.result || '';
  }

  async getFingerprint() {
    if (!this.sessionId) throw new Error('Session not created');
    return this._get(`/api/session/${this.sessionId}/fingerprint`);
  }

  async getCookies() {
    if (!this.sessionId) throw new Error('Session not created');
    const resp = await this._get(`/api/session/${this.sessionId}/cookies`);
    return resp.cookies || '';
  }

  async setCookie(name, value) {
    if (!this.sessionId) throw new Error('Session not created');
    await this._post(`/api/session/${this.sessionId}/cookies`, { name, value });
  }

  async close() {
    if (this.sessionId) {
      try { await this._delete(`/api/session/${this.sessionId}`); } catch (e) {}
      this.sessionId = null;
    }
  }
}

module.exports = { Sandbox };
