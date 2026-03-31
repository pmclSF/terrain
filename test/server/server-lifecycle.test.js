import http from 'node:http';
import { TerrainServer } from '../../src/server/TerrainServer.js';

describe('TerrainServer lifecycle', () => {
  it('stop() resolves when a server instance exists but is not listening', async () => {
    const server = new TerrainServer({ port: 0 });
    server._server = http.createServer();

    await expect(server.stop()).resolves.toBeUndefined();
  });
});
