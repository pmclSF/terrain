import { runDoctor } from '../../src/cli/doctor.js';
import fs from 'fs/promises';
import os from 'os';
import path from 'path';

describe('runDoctor', () => {
  let tmpDir;

  beforeEach(async () => {
    tmpDir = await fs.mkdtemp(path.join(os.tmpdir(), 'terrain-doctor-'));
  });

  afterEach(async () => {
    await fs.rm(tmpDir, { recursive: true, force: true });
  });

  describe('result structure', () => {
    it('should return checks array and summary', async () => {
      const result = await runDoctor(tmpDir);
      expect(Array.isArray(result.checks)).toBe(true);
      expect(result.checks.length).toBeGreaterThan(0);
      expect(typeof result.summary).toBe('object');
      expect(typeof result.summary.pass).toBe('number');
      expect(typeof result.summary.warn).toBe('number');
      expect(typeof result.summary.fail).toBe('number');
      expect(result.summary.total).toBe(result.checks.length);
    });

    it('should set hasFail when there are failures', async () => {
      const result = await runDoctor('/nonexistent/path/that/does/not/exist');
      expect(result.hasFail).toBe(true);
    });

    it('should have properly structured checks', async () => {
      const result = await runDoctor(tmpDir);
      for (const check of result.checks) {
        expect(typeof check.id).toBe('string');
        expect(typeof check.label).toBe('string');
        expect(['PASS', 'WARN', 'FAIL']).toContain(check.status);
        expect(typeof check.detail).toBe('string');
      }
    });
  });

  describe('node version check', () => {
    it('should include a node-version check', async () => {
      const result = await runDoctor(tmpDir);
      const nodeCheck = result.checks.find((c) => c.id === 'node-version');
      expect(nodeCheck).toBeDefined();
      expect(nodeCheck.detail).toContain(process.versions.node);
    });
  });

  describe('target path check', () => {
    it('should pass for a valid directory', async () => {
      const result = await runDoctor(tmpDir);
      const pathCheck = result.checks.find((c) => c.id === 'target-path');
      expect(pathCheck).toBeDefined();
      expect(pathCheck.status).toBe('PASS');
    });

    it('should fail for a non-existent path', async () => {
      const result = await runDoctor('/nonexistent/doctor/test/path');
      const pathCheck = result.checks.find((c) => c.id === 'target-path');
      expect(pathCheck).toBeDefined();
      expect(pathCheck.status).toBe('FAIL');
      expect(pathCheck.detail).toContain('does not exist');
    });

    it('should not run further checks when path is invalid', async () => {
      const result = await runDoctor('/nonexistent/doctor/test/path');
      // Only node-version and target-path should be present
      expect(result.checks.length).toBe(2);
    });
  });

  describe('project type check', () => {
    it('should warn when no package.json exists', async () => {
      const result = await runDoctor(tmpDir);
      const projectCheck = result.checks.find((c) => c.id === 'project-type');
      expect(projectCheck).toBeDefined();
      expect(projectCheck.status).toBe('WARN');
      expect(projectCheck.detail).toContain('No package.json');
    });

    it('should pass and detect frameworks when package.json exists', async () => {
      await fs.writeFile(
        path.join(tmpDir, 'package.json'),
        JSON.stringify({
          devDependencies: { jest: '^29.0.0', cypress: '^13.0.0' },
        })
      );
      const result = await runDoctor(tmpDir);
      const projectCheck = result.checks.find((c) => c.id === 'project-type');
      expect(projectCheck).toBeDefined();
      expect(projectCheck.status).toBe('PASS');
      expect(projectCheck.detail).toContain('jest');
      expect(projectCheck.detail).toContain('cypress');
    });
  });

  describe('test files check', () => {
    it('should warn when no test files found', async () => {
      const result = await runDoctor(tmpDir);
      const testCheck = result.checks.find((c) => c.id === 'test-files');
      expect(testCheck).toBeDefined();
      expect(testCheck.status).toBe('WARN');
      expect(testCheck.detail).toContain('No test files');
    });
  });

  describe('output permissions check', () => {
    it('should pass for writable directories', async () => {
      const result = await runDoctor(tmpDir);
      const permCheck = result.checks.find(
        (c) => c.id === 'output-permissions'
      );
      expect(permCheck).toBeDefined();
      expect(permCheck.status).toBe('PASS');
    });
  });

  describe('Jest ESM check', () => {
    it('should warn when Jest 29 is detected', async () => {
      await fs.writeFile(
        path.join(tmpDir, 'package.json'),
        JSON.stringify({ devDependencies: { jest: '^29.6.0' } })
      );
      const result = await runDoctor(tmpDir);
      const esmCheck = result.checks.find((c) => c.id === 'jest-esm');
      expect(esmCheck).toBeDefined();
      expect(esmCheck.status).toBe('WARN');
      expect(esmCheck.detail).toContain('experimental-vm-modules');
    });

    it('should not produce jest-esm check when no Jest dependency', async () => {
      await fs.writeFile(
        path.join(tmpDir, 'package.json'),
        JSON.stringify({ devDependencies: { vitest: '^1.0.0' } })
      );
      const result = await runDoctor(tmpDir);
      const esmCheck = result.checks.find((c) => c.id === 'jest-esm');
      expect(esmCheck).toBeUndefined();
    });
  });

  describe('summary calculation', () => {
    it('should correctly count pass/warn/fail', async () => {
      const result = await runDoctor(tmpDir);
      const { pass, warn, fail, total } = result.summary;
      expect(pass + warn + fail).toBe(total);
      expect(total).toBe(result.checks.length);
    });
  });
});
