'use strict';

const fs = require('fs');

function parseArgs(argv) {
  const values = {};
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (!argument.startsWith('--')) {
      continue;
    }
    const name = argument.slice(2);
    values[name] = argv[index + 1] || '';
    index += 1;
  }
  return values;
}

function readText(path) {
  try {
    return fs.readFileSync(path, 'utf8');
  } catch {
    return '';
  }
}

function renderReport({ resultText, stderrText, metadata }) {
  const lines = [
    '# Open Code Review Report',
    '',
    `- PR: #${metadata.prNumber || 'unknown'} [${metadata.prTitle || 'unknown'}](${metadata.prUrl || '#'})`,
    `- Reviewed range: \`${metadata.mergeBase || 'unknown'}\` → \`${metadata.headSha || 'unknown'}\``,
    `- OCR exit code: \`${metadata.ocrExitCode || 'unknown'}\``,
  ];

  let result;
  let parseError = '';
  if (!resultText.trim()) {
    result = {};
    parseError = 'result JSON was not generated';
  } else {
    try {
      result = JSON.parse(resultText);
    } catch (error) {
      result = {};
      parseError = error.message;
    }
  }
  if (parseError) {
    result = {};
  }

  const comments = Array.isArray(result.comments) ? result.comments : [];
  lines.push(`- Findings: **${comments.length}**`, '');

  if (comments.length === 0 && !parseError) {
    lines.push('## Result', '', 'No findings were returned by Open Code Review.', '');
  }

  comments.forEach((comment, index) => {
    const line = comment.end_line || comment.start_line || 0;
    const location = line ? `${comment.path || 'unknown'}:${line}` : comment.path || 'unknown';
    lines.push(
      `## ${index + 1}. ${comment.severity || 'unknown'} / ${comment.category || 'other'}`,
      '',
      `- Location: \`${location}\``,
      `- ${comment.content || 'No description returned.'}`,
      '',
    );
    if (comment.suggestion_code) {
      lines.push('```suggestion', String(comment.suggestion_code), '```', '');
    }
  });

  if (parseError || stderrText.trim()) {
    lines.push('## OCR diagnostics', '');
    if (parseError) {
      lines.push(`- Result JSON could not be parsed: ${parseError}`, '');
    }
    if (stderrText.trim()) {
      lines.push('```text', stderrText.trim().slice(0, 12000), '```', '');
    }
  }

  return `${lines.join('\n')}\n`;
}

function main(argv = process.argv.slice(2), environment = process.env) {
  const args = parseArgs(argv);
  if (!args.result || !args.stderr || !args.output) {
    throw new Error('Usage: render-ocr-history-report.js --result <path> --stderr <path> --output <path>');
  }

  const report = renderReport({
    resultText: readText(args.result),
    stderrText: readText(args.stderr),
    metadata: {
      prNumber: environment.OCR_PR_NUMBER,
      prTitle: environment.OCR_PR_TITLE,
      prUrl: environment.OCR_PR_URL,
      mergeBase: environment.OCR_MERGE_BASE,
      headSha: environment.OCR_HEAD_SHA,
      ocrExitCode: environment.OCR_EXIT_CODE,
    },
  });
  fs.writeFileSync(args.output, report);
}

if (require.main === module) {
  try {
    main();
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}

module.exports = { main, renderReport };
