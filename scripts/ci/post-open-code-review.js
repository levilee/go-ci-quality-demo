const fs = require('node:fs');
const crypto = require('node:crypto');

const SUMMARY_MARKER = '<!-- open-code-review-summary -->';
const INLINE_MARKER_PREFIX = '<!-- open-code-review-inline:';

function text(value) {
  return typeof value === 'string' ? value.trim() : '';
}

function commentContent(comment) {
  return text(comment.content || comment.message || comment.comment);
}

function classifyComments(comments) {
  const inline = [];
  const summary = [];

  for (const raw of Array.isArray(comments) ? comments : []) {
    const path = text(raw.path);
    const content = commentContent(raw);
    const endLine = Number(raw.end_line || raw.line || raw.start_line || 0);
    const startLine = Number(raw.start_line || endLine);

    if (!path || !content || !Number.isInteger(startLine) || !Number.isInteger(endLine) || startLine < 1 || endLine < startLine) {
      summary.push({
        path: path || '(unknown file)',
        content: content || 'Open Code Review returned a finding without content.',
        suggestionCode: text(raw.suggestion_code),
      });
      continue;
    }

    inline.push({
      path,
      content,
      startLine,
      endLine,
      suggestionCode: text(raw.suggestion_code),
    });
  }

  return { inline, summary };
}

function commentMarker(item) {
  const digest = crypto.createHash('sha256')
    .update(JSON.stringify([item.path, item.startLine, item.endLine, item.content]))
    .digest('hex')
    .slice(0, 16);
  return `${INLINE_MARKER_PREFIX}${digest} -->`;
}

function buildInlinePayload(item) {
  const suggestion = item.suggestionCode
    ? `\n\n**Suggestion:**\n\n\`\`\`suggestion\n${item.suggestionCode}\n\`\`\``
    : '';
  const marker = commentMarker(item);
  const payload = {
    path: item.path,
    line: item.endLine,
    side: 'RIGHT',
    body: `${marker}\n${item.content}${suggestion}`,
  };

  if (item.startLine < item.endLine) {
    payload.start_line = item.startLine;
    payload.start_side = 'RIGHT';
  }

  return payload;
}

function buildSummaryBody({ pullNumber, headSha, inlinePosted, skipped, summaryItems, warnings, failure }) {
  const lines = [
    SUMMARY_MARKER,
    '## OpenCodeReview',
    `Review for PR #${pullNumber} at head \`${headSha}\`.`,
  ];

  if (failure) {
    lines.push(`⚠️ Review did not complete: ${failure}`);
  } else if (inlinePosted === 0 && skipped === 0 && summaryItems.length === 0) {
    lines.push('✅ No comments generated. Looks good to me.');
  } else {
    lines.push(`- Inline comments posted: ${inlinePosted}`);
    lines.push(`- Existing inline comments skipped: ${skipped}`);
    lines.push(`- Findings in summary: ${summaryItems.length}`);
  }

  for (const warning of warnings) {
    lines.push(`- ⚠️ ${warning}`);
  }

  for (const item of summaryItems) {
    lines.push('', `### ${item.path}`, item.content);
    if (item.suggestionCode) {
      lines.push('', '**Suggestion:**', '```suggestion', item.suggestionCode, '```');
    }
  }

  return lines.join('\n');
}

async function listAll(github, method, params) {
  const items = [];
  for (let page = 1; page <= 50; page += 1) {
    const response = await method({ ...params, per_page: 100, page });
    items.push(...(response.data || []));
    if ((response.data || []).length < 100) break;
  }
  return items;
}

async function upsertSummary(github, owner, repo, pullNumber, body) {
  const comments = await listAll(github, github.rest.issues.listComments, {
    owner,
    repo,
    issue_number: pullNumber,
  });
  const existing = comments.find((comment) => text(comment.body).includes(SUMMARY_MARKER));

  if (existing) {
    await github.rest.issues.updateComment({ owner, repo, comment_id: existing.id, body });
  } else {
    await github.rest.issues.createComment({ owner, repo, issue_number: pullNumber, body });
  }
}

async function postOpenCodeReview({ github, owner, repo, pullNumber, headSha, resultPath, stderrPath, ocrExitCode }) {
  const stderr = fs.existsSync(stderrPath) ? text(fs.readFileSync(stderrPath, 'utf8')) : '';
  const failure = Number(ocrExitCode) === 0 ? '' : (stderr || `ocr review exited with code ${ocrExitCode}`);
  let result = {};

  if (!failure && fs.existsSync(resultPath)) {
    try {
      result = JSON.parse(fs.readFileSync(resultPath, 'utf8'));
    } catch (error) {
      await upsertSummary(github, owner, repo, pullNumber, buildSummaryBody({
        pullNumber,
        headSha,
        inlinePosted: 0,
        skipped: 0,
        summaryItems: [],
        warnings: [],
        failure: `failed to parse OCR JSON: ${error.message}`,
      }));
      return;
    }
  }

  const classified = classifyComments(result.comments);
  const warnings = Array.isArray(result.warnings) ? result.warnings.map(text).filter(Boolean) : [];
  const existingComments = await listAll(github, github.rest.pulls.listReviewComments, {
    owner,
    repo,
    pull_number: pullNumber,
  });
  const existingMarkers = new Set();

  for (const comment of existingComments) {
    const match = text(comment.body).match(/<!-- open-code-review-inline:[0-9a-f]{16} -->/);
    if (match) existingMarkers.add(match[0]);
  }

  const freshInline = classified.inline.filter((item) => !existingMarkers.has(commentMarker(item)));
  const skipped = classified.inline.length - freshInline.length;
  let inlinePosted = 0;
  const summaryItems = [...classified.summary];

  if (!failure && freshInline.length > 0) {
    try {
      await github.rest.pulls.createReview({
        owner,
        repo,
        pull_number: pullNumber,
        commit_id: headSha,
        event: 'COMMENT',
        body: '',
        comments: freshInline.map(buildInlinePayload),
      });
      inlinePosted = freshInline.length;
    } catch (error) {
      summaryItems.push(...freshInline.map((item) => ({
        path: item.path,
        content: `${item.content} (inline comment could not be posted: ${error.message})`,
        suggestionCode: item.suggestionCode,
      })));
      warnings.push('Some findings could not be attached to diff lines and were moved to the summary.');
    }
  }

  await upsertSummary(github, owner, repo, pullNumber, buildSummaryBody({
    pullNumber,
    headSha,
    inlinePosted,
    skipped,
    summaryItems,
    warnings,
    failure,
  }));
}

module.exports = {
  SUMMARY_MARKER,
  classifyComments,
  buildInlinePayload,
  buildSummaryBody,
  commentMarker,
  postOpenCodeReview,
};
