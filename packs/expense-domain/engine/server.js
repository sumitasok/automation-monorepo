/**
 * Expense Domain HTTP Server
 * Exposes ExpenseEngine API as REST endpoints
 */

const http = require('http');
const url = require('url');
const ExpenseEngine = require('./api.js');

class ExpenseServer {
  constructor(configPath, port = 3100) {
    this.configPath = configPath;
    this.port = port;
    this.engine = new ExpenseEngine(configPath);
    this.server = null;
  }

  /**
   * Start the server
   */
  async start() {
    // Initialize engine
    await this.engine.initialize();
    await this.engine.start();

    // Create HTTP server
    this.server = http.createServer((req, res) => {
      this._handleRequest(req, res);
    });

    return new Promise((resolve, reject) => {
      this.server.listen(this.port, (err) => {
        if (err) reject(err);
        console.log(`Expense Domain Server listening on port ${this.port}`);
        resolve();
      });
    });
  }

  /**
   * Stop the server
   */
  async stop() {
    if (this.server) {
      return new Promise((resolve) => {
        this.server.close(resolve);
      });
    }
  }

  // ============ Request Handling ============

  async _handleRequest(req, res) {
    const parsedUrl = url.parse(req.url, true);
    const pathname = parsedUrl.pathname;
    const query = parsedUrl.query;

    // CORS headers
    res.setHeader('Access-Control-Allow-Origin', '*');
    res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PATCH, DELETE, OPTIONS');
    res.setHeader('Access-Control-Allow-Headers', 'Content-Type');

    if (req.method === 'OPTIONS') {
      res.writeHead(200);
      res.end();
      return;
    }

    try {
      // Route to handler
      if (pathname === '/api/expense-domain/expenses' && req.method === 'GET') {
        await this._handleGetExpenses(req, res, query);
      } else if (pathname.match(/^\/api\/expense-domain\/expenses\/[^/]+$/) && req.method === 'GET') {
        const id = pathname.split('/').pop();
        await this._handleGetExpense(req, res, id);
      } else if (pathname === '/api/expense-domain/expenses' && req.method === 'POST') {
        await this._handleCreateExpense(req, res);
      } else if (pathname.match(/^\/api\/expense-domain\/expenses\/[^/]+$/) && req.method === 'PATCH') {
        const id = pathname.split('/').pop();
        await this._handleUpdateExpense(req, res, id);
      } else if (pathname.match(/^\/api\/expense-domain\/expenses\/[^/]+$/) && req.method === 'DELETE') {
        const id = pathname.split('/').pop();
        await this._handleDeleteExpense(req, res, id);
      } else if (pathname === '/api/expense-domain/rules' && req.method === 'GET') {
        await this._handleGetRules(req, res, query);
      } else if (pathname === '/api/expense-domain/rules' && req.method === 'POST') {
        await this._handleCreateRule(req, res);
      } else if (pathname.match(/^\/api\/expense-domain\/rules\/[^/]+$/) && req.method === 'PATCH') {
        const id = pathname.split('/').pop();
        await this._handleUpdateRule(req, res, id);
      } else if (pathname.match(/^\/api\/expense-domain\/rules\/[^/]+$/) && req.method === 'DELETE') {
        const id = pathname.split('/').pop();
        await this._handleDeleteRule(req, res, id);
      } else if (pathname.match(/^\/api\/expense-domain\/sources\/[^/]+\/status$/)) {
        const source = pathname.split('/')[4];
        await this._handleSourceStatus(req, res, source);
      } else if (pathname.match(/^\/api\/expense-domain\/sources\/[^/]+\/write-back$/)) {
        const source = pathname.split('/')[4];
        await this._handleWriteBack(req, res, source);
      } else if (pathname === '/health') {
        this._sendJson(res, 200, { status: 'ok', domain: 'expense-domain' });
      } else {
        this._sendError(res, 404, 'Not Found');
      }
    } catch (error) {
      console.error('Error handling request:', error);
      this._sendError(res, 500, error.message);
    }
  }

  async _handleGetExpenses(req, res, query) {
    const expenses = await this.engine.getExpenses(query);
    this._sendJson(res, 200, expenses);
  }

  async _handleGetExpense(req, res, id) {
    const expense = await this.engine.getExpense(id);
    this._sendJson(res, 200, expense);
  }

  async _handleCreateExpense(req, res) {
    const body = await this._readBody(req);
    const data = JSON.parse(body);
    const expense = await this.engine.createExpense(data);
    this._sendJson(res, 201, expense);
  }

  async _handleUpdateExpense(req, res, id) {
    const body = await this._readBody(req);
    const updates = JSON.parse(body);
    const expense = await this.engine.updateExpense(id, updates);
    this._sendJson(res, 200, expense);
  }

  async _handleDeleteExpense(req, res, id) {
    await this.engine.deleteExpense(id);
    this._sendJson(res, 204, null);
  }

  async _handleGetRules(req, res, query) {
    const rules = await this.engine.getRules(query);
    this._sendJson(res, 200, rules);
  }

  async _handleCreateRule(req, res) {
    const body = await this._readBody(req);
    const data = JSON.parse(body);
    const rule = await this.engine.createRule(data);
    this._sendJson(res, 201, rule);
  }

  async _handleUpdateRule(req, res, id) {
    const body = await this._readBody(req);
    const updates = JSON.parse(body);
    const rule = await this.engine.updateRule(id, updates);
    this._sendJson(res, 200, rule);
  }

  async _handleDeleteRule(req, res, id) {
    await this.engine.deleteRule(id);
    this._sendJson(res, 204, null);
  }

  async _handleSourceStatus(req, res, source) {
    const status = await this.engine.getSourceStatus(source);
    this._sendJson(res, 200, status);
  }

  async _handleWriteBack(req, res, source) {
    const body = await this._readBody(req);
    const data = JSON.parse(body);
    const result = await this.engine.writeBackToSource(source, data);
    this._sendJson(res, 200, result);
  }

  // ============ Utilities ============

  _readBody(req) {
    return new Promise((resolve, reject) => {
      let body = '';
      req.on('data', (chunk) => {
        body += chunk;
      });
      req.on('end', () => {
        resolve(body);
      });
      req.on('error', reject);
    });
  }

  _sendJson(res, statusCode, data) {
    res.writeHead(statusCode, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify(data));
  }

  _sendError(res, statusCode, message) {
    res.writeHead(statusCode, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: message }));
  }
}

module.exports = ExpenseServer;

// Allow running as standalone server
if (require.main === module) {
  const configPath = process.env.CONFIG_PATH || process.env.HOME + '/automation-monorepo-config';
  const port = process.env.PORT || 3100;

  const server = new ExpenseServer(configPath, port);
  server.start().catch((err) => {
    console.error('Failed to start server:', err);
    process.exit(1);
  });

  // Graceful shutdown
  process.on('SIGTERM', async () => {
    console.log('SIGTERM received, shutting down');
    await server.stop();
    process.exit(0);
  });
}
