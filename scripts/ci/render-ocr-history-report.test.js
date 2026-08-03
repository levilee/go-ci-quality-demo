'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const test = require('node:test');
const { main, renderReport } = require('./render-ocr-history-report');

const metadata = {
  prNumber: '12',
  prTitle: 'Test divide operation',
  prUrl: 'https://github.com/example/repo/pull/12',
  mergeBase: 'base123',
  headSha: 'head456',
  ocrExitCode: '0',
};

test('renders a finding with location and suggestion', () => {
  const report = renderReport({
    metadata,
    stderrText: '',
    resultText: JSON.stringify({
      comments: [{
        severity: 'high',
        category: 'bug',
        path: 'internal/httpapi/handler.go',
        end_line: 70,
        content: 'Division by zero can panic the request handler.',
        suggestion_code: 'if right == 0 { return }',
      }],
    }),
  });

  assert.match(report, /Findings: \*\*1\*\*/);
  assert.match(report, /internal\/httpapi\/handler\.go:70/);
  assert.match(report, /Division by zero/);
  assert.match(report, /```suggestion/);
});

test('renders an explicit no-findings result', () => {
  const report = renderReport({ metadata, stderrText: '', resultText: '{"comments":[]}' });

  assert.match(report, /Findings: \*\*0\*\*/);
  assert.match(report, /No findings were returned/);
});

test('renders diagnostics when OCR JSON is invalid', () => {
  const report = renderReport({ metadata: { ...metadata, ocrExitCode: '1' }, stderrText: 'LLM unavailable', resultText: '{invalid' });

  assert.match(report, /could not be parsed/);
  assert.match(report, /LLM unavailable/);
});

test('renders diagnostics when OCR JSON is missing', () => {
  const report = renderReport({ metadata: { ...metadata, ocrExitCode: '127' }, stderrText: 'ocr: command not found', resultText: '' });

  assert.match(report, /result JSON was not generated/);
  assert.match(report, /ocr: command not found/);
});

test('writes a report through the command interface', () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'ocr-report-'));
  const resultPath = path.join(directory, 'result.json');
  const stderrPath = path.join(directory, 'stderr.log');
  const outputPath = path.join(directory, 'report.md');
  fs.writeFileSync(resultPath, '{"comments":[]}');
  fs.writeFileSync(stderrPath, '');

  main(['--result', resultPath, '--stderr', stderrPath, '--output', outputPath], metadata);

  assert.match(fs.readFileSync(outputPath, 'utf8'), /Open Code Review Report/);
});
