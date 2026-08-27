// bes Node.js SDK (skeleton)
// Requires: @grpc/grpc-js

class Sandbox {
  constructor(serverAddr = 'localhost:50051', opts = {}) {
    this.serverAddr = serverAddr;
    this.sessionId = null;
    this._client = null;
    this._connect(opts);
  }

  _connect(opts) {
    // TODO: const grpc = require('@grpc/grpc-js');
    // TODO: const proto = require('./bes_pb2');
    // TODO: this._client = new proto.SandboxService(this.serverAddr, grpc.credentials.createInsecure());
    // TODO: this._createSession(opts);
    console.log(`[bes] connecting to ${this.serverAddr} (gRPC client: TODO)`);
  }

  async eval(code) {
    // TODO: return new Promise((resolve, reject) => {
    //   this._client.eval({sessionId: this.sessionId, code}, (err, resp) => { ... });
    // });
    throw new Error('gRPC client not yet implemented (Phase 6)');
  }

  async loadScript(name, content) {
    throw new Error('gRPC client not yet implemented (Phase 6)');
  }

  async call(functionName, ...args) {
    throw new Error('gRPC client not yet implemented (Phase 6)');
  }

  async close() {
    if (this.sessionId) {
      console.log(`[bes] session ${this.sessionId} closed`);
      this.sessionId = null;
    }
  }
}

module.exports = { Sandbox };
