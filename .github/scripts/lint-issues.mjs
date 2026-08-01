/**
 * Lint Issues Automation Script
 *
 * Parses lint findings from golangci-lint JSON output, classifies by severity,
 * and creates/updates/closes GitHub Issues based on the lint-governance.yml config.
 *
 * Usage:
 *   node lint-issues.mjs \
 *     --report <path-to-golangci-lint-json> \
 *     --config <path-to-lint-governance.yml> \
 *     --token <github-token> \
 *     --repo <owner/repo> \
 *     --run-url <github-run-url>
 *
 * Requires: @actions/github, @actions/core, js-yaml
 */

import { readFileSync, existsSync } from 'fs';
import { resolve } from 'path';
import { createHash } from 'crypto';

// Minimal YAML parser — no external deps needed for the CI runner.
// Supports nested maps and arrays of scalars.
function parseYaml(text) {
  const result = {};
  const stack = [{ obj: result, indent: -1, arrayTarget: null }];

  for (const raw of text.split('\n')) {
    const trimmed = raw.trimEnd();
    if (trimmed.trim() === '' || trimmed.trim().startsWith('#')) continue;

    const indent = trimmed.length - trimmed.trimStart().length;
    const content = trimmed.trimStart();

    // Array item: "- value"
    if (content.startsWith('- ')) {
      const val = content.slice(2).trim();
      while (stack.length > 0 && stack[stack.length - 1].indent >= indent) stack.pop();
      const entry = stack[stack.length - 1];
      if (entry.arrayTarget) {
        const { obj, key } = entry.arrayTarget;
        if (!Array.isArray(obj[key])) obj[key] = [];
        obj[key].push(val);
      }
      continue;
    }

    const colonIdx = content.indexOf(':');
    if (colonIdx === -1) continue;

    const key = content.slice(0, colonIdx).trim();
    const rest = content.slice(colonIdx + 1).trim();

    // Pop stack to correct indent level
    while (stack.length > 1 && stack[stack.length - 1].indent >= indent) stack.pop();
    const parent = stack[stack.length - 1];

    if (rest === '') {
      // Nested object — remember its parent key so later "- " items attach here
      const newObj = {};
      parent.obj[key] = newObj;
      stack.push({ obj: newObj, indent, arrayTarget: { obj: parent.obj, key } });
    } else {
      // Leaf value
      let val = rest;
      if ((val.startsWith('"') && val.endsWith('"')) || (val.startsWith("'") && val.endsWith("'"))) {
        val = val.slice(1, -1);
      }
      parent.obj[key] = val;
      stack[stack.length - 1].arrayTarget = null;
    }
  }
  return result;
}

// Parse CLI args
function parseArgs() {
  const args = {};
  for (let i = 2; i < process.argv.length; i++) {
    const arg = process.argv[i];
    if (arg.startsWith('--')) {
      const key = arg.slice(2);
      const val = process.argv[++i];
      args[key] = val;
    }
  }
  return args;
}

// Compute fingerprint for a finding
// golangci-lint v2 JSON format:
//   { "FromLinter": "gosec", "Text": "...", "Severity": "medium", "Pos": { "Filename": "main.go", "Line": 11, "Column": 9 } }
function fingerprint(finding, repo) {
  const path = finding.Pos?.Filename || finding.file || finding.path || 'unknown';
  const rule = finding.FromLinter || finding.linter || finding.rule || 'unknown';
  const line = finding.Pos?.Line || finding.line || 0;
  // Include the message text so distinct findings at the same location don't collide
  const msg = finding.Text || finding.text || finding.message || '';
  const raw = `${repo}:${path}:${rule}:${line}:${msg.slice(0, 60)}`;
  return createHash('md5').update(raw).digest('hex').slice(0, 12);
}

// Map golangci-lint finding to our P-levels.
// golangci-lint v2 uses severity "error" | "warning" | "info" (defaults per linter);
// gosec security findings are treated as P0 regardless of reported severity.
function classifySeverity(finding) {
  const rule = (finding.FromLinter || finding.linter || finding.rule || '').toLowerCase();

  // Gosec security rules → P0
  if (rule === 'gosec') {
    return { level: 'P0', label: 'Critical' };
  }

  const sev = (finding.Severity || '').toLowerCase();
  if (sev === 'error') return { level: 'P1', label: 'High' };
  if (sev === 'warning' || sev === 'medium') return { level: 'P2', label: 'Medium' };
  return { level: 'P3', label: 'Low' };
}

// Build issue body from a finding
function buildIssueBody(finding, severity, repo, runUrl) {
  const rule = finding.FromLinter || finding.linter || finding.rule || 'unknown';
  const path = finding.Pos?.Filename || finding.file || finding.path || 'unknown';
  const line = finding.Pos?.Line || finding.line || 0;
  const msg = finding.Text || finding.text || finding.message || 'No description';

  return `## Summary

**${rule}**: ${msg}

**Severity**: ${severity.level} - ${severity.label}
**Linter Rule**: \`${rule}\`
**File**: \`${path}:${line}\`
**CI Run**: ${runUrl}

### Problem

${msg}

### Suggested Fix

Review the code at \`${path}:${line}\` and address the lint finding.

### Acceptance Criteria

- [ ] No new lint findings of this type in CI
- [ ] All related lint findings are resolved
`;
}

// Main
async function main() {
  const args = parseArgs();
  const reportPath = args.report || resolve('.', 'golangci-lint-report.json');
  const configPath = args.config || resolve('.github', 'lint-governance.yml');
  const repo = args.repo || process.env.GITHUB_REPOSITORY || 'owner/repo';
  const runUrl = args.run_url || process.env.GITHUB_SERVER_URL + '/' + process.env.GITHUB_REPOSITORY + '/actions/runs/' + (process.env.GITHUB_RUN_ID || '0');
  const token = args.token || process.env.GITHUB_TOKEN || '';

  if (!token) {
    console.log('No GITHUB_TOKEN available — skipping issue automation');
    return;
  }

  // Read config
  let config = {};
  if (existsSync(configPath)) {
    const raw = readFileSync(configPath, 'utf-8');
    config = parseYaml(raw);
  }

  // Read lint report
  if (!existsSync(reportPath)) {
    console.log(`Report not found: ${reportPath} — skipping`);
    return;
  }

  const report = JSON.parse(readFileSync(reportPath, 'utf-8'));
  const findings = report.Issues || report.issues || [];

  // Fetch existing lint issues
  const { default: github } = await import('@actions/github');
  const octokit = github.getOctokit(token);
  const [owner, repoName] = repo.split('/');

  // Existing lint-labeled issues
  const existingIssues = [];
  try {
    const resp = await octokit.rest.issues.listForRepo({
      owner, repo: repoName,
      labels: 'lint',
      state: 'all',
      per_page: 100,
    });
    existingIssues.push(...resp.data);
  } catch (e) {
    console.log(`Warning: could not list issues: ${e.message}`);
  }

  // Build index by fingerprint label
  const issueIndex = new Map();
  for (const issue of existingIssues) {
    for (const label of issue.labels) {
      if (typeof label === 'string' && label.startsWith('lint-fp-')) {
        issueIndex.set(label, issue);
      } else if (label.name && label.name.startsWith('lint-fp-')) {
        issueIndex.set(label.name, issue);
      }
    }
  }

  const autoCreate = config.auto_create_issues || ['P0', 'P1', 'P2'];
  const processedFingerprints = new Set();

  for (const finding of findings) {
    const fp = fingerprint(finding, repo);
    const severity = classifySeverity(finding);
    const fpLabel = `lint-fp-${fp}`;

    if (!autoCreate.includes(severity.level)) {
      continue; // P3 and below: no issues
    }

    processedFingerprints.add(fp);

    const existingIssue = issueIndex.get(fpLabel);

    if (existingIssue) {
      // Issue exists — update if needed
      if (existingIssue.state === 'closed') {
        try {
          await octokit.rest.issues.createComment({
            owner, repo: repoName,
            issue_number: existingIssue.number,
            body: `🔁 This lint finding has reappeared in CI run: ${runUrl}`,
          });
          await octokit.rest.issues.update({
            owner, repo: repoName,
            issue_number: existingIssue.number,
            state: 'open',
          });
          console.log(`Reopened issue #${existingIssue.number} (${fpLabel})`);
        } catch (e) {
          console.log(`Warning: could not reopen issue #${existingIssue.number}: ${e.message}`);
        }
      } else {
        // Still open — add a comment with latest run info
        try {
          await octokit.rest.issues.createComment({
            owner, repo: repoName,
            issue_number: existingIssue.number,
            body: `🔄 Still detected in CI run: ${runUrl}`,
          });
          console.log(`Updated issue #${existingIssue.number} (${fpLabel})`);
        } catch (e) {
          console.log(`Warning: could not update issue #${existingIssue.number}: ${e.message}`);
        }
      }
    } else {
      // Create new issue
      try {
        const severityLabel = config.issue_labels?.[severity.level] || `lint/${severity.level.toLowerCase()}`;
        const body = buildIssueBody(finding, severity, repo, runUrl);

        await octokit.rest.issues.create({
          owner, repo: repoName,
          title: `[${severity.level}] ${finding.FromLinter || finding.linter || 'lint'}: ${(finding.Text || finding.text || finding.message || '').slice(0, 80)}`,
          body,
          labels: ['lint', severityLabel, fpLabel],
        });
        console.log(`Created issue for ${fpLabel}`);
      } catch (e) {
        console.log(`Warning: could not create issue for ${fpLabel}: ${e.message}`);
      }
    }
  }

  // Close issues for resolved findings
  for (const [fpLabel, issue] of issueIndex) {
    const fp = fpLabel.replace('lint-fp-', '');
    if (!processedFingerprints.has(fp) && issue.state === 'open') {
      try {
        await octokit.rest.issues.createComment({
          owner, repo: repoName,
          issue_number: issue.number,
          body: `✅ This lint finding has been resolved. CI run: ${runUrl}`,
        });
        await octokit.rest.issues.update({
          owner, repo: repoName,
          issue_number: issue.number,
          state: 'closed',
        });
        console.log(`Closed issue #${issue.number} (${fpLabel})`);
      } catch (e) {
        console.log(`Warning: could not close issue #${issue.number}: ${e.message}`);
      }
    }
  }

  console.log('Issue automation complete');
}

main().catch(err => {
  console.error('Fatal error:', err.message);
  process.exit(0); // Don't fail CI on issue automation errors
});