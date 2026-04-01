import { PipelineConverter } from '../../src/core/PipelineConverter.js';
import { BaseConverter } from '../../src/core/BaseConverter.js';

// Minimal framework definitions for testing
function makeDetect(keyword, score) {
  return (source) => (source.includes(keyword) ? score : 0);
}

function makeParse() {
  return (source) => ({
    type: 'TestFile',
    children: [{ type: 'RawCode', value: source }],
  });
}

function makeEmit() {
  return (ir) => {
    const raw = ir.children.find((c) => c.type === 'RawCode');
    return raw ? raw.value : '';
  };
}

const fakeSource = {
  name: 'fakesrc',
  language: 'javascript',
  detect: makeDetect('fakesrc', 80),
  parse: makeParse(),
  emit: makeEmit(),
  imports: ["import { fakesrc } from 'fakesrc';"],
  paradigm: 'unit',
};

const fakeTarget = {
  name: 'faketgt',
  language: 'javascript',
  detect: makeDetect('faketgt', 80),
  parse: makeParse(),
  emit: makeEmit(),
  imports: ["import { faketgt } from 'faketgt';"],
  paradigm: 'unit',
};

describe('PipelineConverter', () => {
  let converter;

  beforeEach(() => {
    converter = new PipelineConverter('fakesrc', 'faketgt', [
      fakeSource,
      fakeTarget,
    ]);
  });

  describe('constructor', () => {
    it('should extend BaseConverter', () => {
      expect(converter).toBeInstanceOf(BaseConverter);
    });

    it('should set sourceFramework and targetFramework', () => {
      expect(converter.sourceFramework).toBe('fakesrc');
      expect(converter.targetFramework).toBe('faketgt');
    });

    it('should initialize with zero stats', () => {
      expect(converter.stats.conversions).toBe(0);
      expect(converter.stats.warnings).toEqual([]);
      expect(converter.stats.errors).toEqual([]);
    });

    it('should accept options and pass them to BaseConverter', () => {
      const custom = new PipelineConverter('fakesrc', 'faketgt', [fakeSource, fakeTarget], {
        preserveComments: false,
      });
      expect(custom.options.preserveComments).toBe(false);
    });
  });

  describe('convert', () => {
    it('should convert content and increment stats', async () => {
      const result = await converter.convert('fakesrc code here');
      expect(typeof result).toBe('string');
      expect(converter.stats.conversions).toBe(1);
    });

    it('should increment stats on each call', async () => {
      await converter.convert('fakesrc input 1');
      await converter.convert('fakesrc input 2');
      expect(converter.stats.conversions).toBe(2);
    });

    it('should return a string', async () => {
      const result = await converter.convert('fakesrc test content');
      expect(typeof result).toBe('string');
    });
  });

  describe('getLastReport', () => {
    it('should return null before any conversion', () => {
      expect(converter.getLastReport()).toBeNull();
    });

    it('should return a report after conversion', async () => {
      await converter.convert('fakesrc test content');
      const report = converter.getLastReport();
      expect(report).not.toBeNull();
      expect(typeof report).toBe('object');
    });
  });

  describe('convertConfig', () => {
    it('should throw on non-existent config file', async () => {
      await expect(
        converter.convertConfig('/nonexistent/config.js')
      ).rejects.toThrow();
    });
  });
});
