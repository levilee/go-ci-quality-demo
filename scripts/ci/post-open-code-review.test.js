const assert = require('node:assert/strict');
const test = require('node:test');

const {
  SUMMARY_MARKER,
  classifyComments,
  buildInlinePayload,
  buildSummaryBody,
  commentMarker,
} = require('./post-open-code-review.js');

test('classifies line comments as inline and missing-line comments as summary', () => {
  const classified = classifyComments([
    { path: 'internal/httpapi/handler.go', content: 'Use a typed error.', start_line: 12, end_line: 12 },
    { path: 'README.md', content: 'Document the new behavior.' },
  ]);

  assert.equal(classified.inline.length, 1);
  assert.equal(classified.summary.length, 1);
  assert.equal(classified.inline[0].path, 'internal/httpapi/handler.go');
});

test('builds a GitHub right-side inline comment with a deterministic marker', () => {
  const item = {
    path: 'internal/httpapi/handler.go',
    content: 'Use a typed error.',
    startLine: 12,
    endLine: 14,
  };
  const marker = commentMarker(item);
  const payload = buildInlinePayload(item);

  assert.match(marker, /^<!-- open-code-review-inline:[0-9a-f]{16} -->$/);
  assert.deepEqual(payload, {
    path: item.path,
    start_line: 12,
    start_side: 'RIGHT',
    line: 14,
    side: 'RIGHT',
    body: `${marker}\n${item.content}`,
  });
});

test('builds a stable summary body for findings and warnings', () => {
  const body = buildSummaryBody({
    pullNumber: 7,
    headSha: 'abc123',
    inlinePosted: 1,
    skipped: 2,
    summaryItems: [{ path: 'README.md', content: 'Document the endpoint.' }],
    warnings: ['One finding had no line information.'],
    failure: '',
  });

  assert.ok(body.includes(SUMMARY_MARKER));
  assert.match(body, /PR #7/);
  assert.match(body, /abc123/);
  assert.match(body, /README\.md/);
  assert.match(body, /One finding had no line information/);
});
