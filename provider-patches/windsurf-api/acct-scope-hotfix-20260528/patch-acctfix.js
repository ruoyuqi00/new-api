const fs = require('fs');

const path = '/app/src/handlers/chat.js';
let source = fs.readFileSync(path, 'utf8');

const beforePoolCtx = 'reuseEnabled ? { reuseEntry, lsPort: ls.port, apiKey: acct.apiKey, callerKey, cachePolicy, fpOpts, aliasModelKey: context.__aliasModelKey || null } : null,';
const afterPoolCtx = 'reuseEnabled ? { reuseEntry, lsPort: ls.port, apiKey: acct.apiKey, accountId: acct.id, callerKey, cachePolicy, fpOpts, aliasModelKey: context.__aliasModelKey || null } : null,';

const beforeSticky = `      if (poolCtx.callerKey && isStickyEnabled() && acct) {
        setStickyBinding(poolCtx.callerKey, modelKey, acct.id, acct.apiKey);
      }`;
const afterSticky = `      if (poolCtx.callerKey && isStickyEnabled() && poolCtx.accountId && poolCtx.apiKey) {
        setStickyBinding(poolCtx.callerKey, modelKey, poolCtx.accountId, poolCtx.apiKey);
      }`;

let changed = false;

if (source.includes(beforePoolCtx)) {
  source = source.replace(beforePoolCtx, afterPoolCtx);
  changed = true;
} else if (!source.includes(afterPoolCtx)) {
  throw new Error('WindsurfAPI acct hotfix: poolCtx snippet not found; upstream likely changed and needs review');
}

if (source.includes(beforeSticky)) {
  source = source.replace(beforeSticky, afterSticky);
  changed = true;
} else if (!source.includes(afterSticky)) {
  throw new Error('WindsurfAPI acct hotfix: sticky binding snippet not found; upstream likely changed and needs review');
}

fs.writeFileSync(path, source);
console.log(changed ? 'WindsurfAPI acct hotfix applied' : 'WindsurfAPI acct hotfix already present');
