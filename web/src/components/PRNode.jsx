import { useState } from 'react';
import StatusBadge from './StatusBadge.jsx';
import ConflictNote from './ConflictNote.jsx';

function relativeTime(timestamp) {
  if (!timestamp) return 'never';

  const date = new Date(timestamp);
  if (isNaN(date.getTime())) return 'never';

  const diffMs = Date.now() - date.getTime();
  const diffSec = Math.round(diffMs / 1000);

  const rtf = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });

  if (Math.abs(diffSec) < 60) return rtf.format(-diffSec, 'second');
  const diffMin = Math.round(diffSec / 60);
  if (Math.abs(diffMin) < 60) return rtf.format(-diffMin, 'minute');
  const diffHr = Math.round(diffMin / 60);
  if (Math.abs(diffHr) < 24) return rtf.format(-diffHr, 'hour');
  const diffDay = Math.round(diffHr / 24);
  return rtf.format(-diffDay, 'day');
}

const ghostBtnBase = {
  padding: '3px 10px',
  fontSize: '12px',
  fontWeight: '500',
  borderRadius: '5px',
  border: '1px solid',
  cursor: 'pointer',
  background: 'transparent',
  lineHeight: '1.4',
};

export default function PRNode({ entry, repoOwner, repoName, parentBranchName, stackID, apiUrl, onRefresh }) {
  const [removing, setRemoving] = useState(false);
  const [marking, setMarking] = useState(false);

  const prUrl = `https://github.com/${repoOwner}/${repoName}/pull/${entry.pr_number}`;
  const synced = relativeTime(entry.last_synced);
  const isMerged = entry.status === 'merged';

  async function handleRemove() {
    if (!window.confirm(`Remove PR #${entry.pr_number} from this stack?`)) return;
    setRemoving(true);
    try {
      await fetch(`${apiUrl}/api/stacks/${stackID}/entries/${entry.pr_number}`, { method: 'DELETE' });
      onRefresh();
    } finally {
      setRemoving(false);
    }
  }

  async function handleMarkMerged() {
    if (!window.confirm(`Mark PR #${entry.pr_number} as merged? This will retarget the child PR's base branch and trigger a cascade sync.`)) return;
    setMarking(true);
    try {
      await fetch(`${apiUrl}/api/stacks/${stackID}/entries/${entry.pr_number}/merged`, { method: 'POST' });
      onRefresh();
    } finally {
      setMarking(false);
    }
  }

  return (
    <div
      style={{
        background: '#ffffff',
        border: '1px solid #e2e8f0',
        borderRadius: '8px',
        padding: '12px 16px',
        boxShadow: '0 1px 3px rgba(0,0,0,0.06)',
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: '12px',
          flexWrap: 'wrap',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', minWidth: 0 }}>
          <a
            href={prUrl}
            target="_blank"
            rel="noopener noreferrer"
            style={{
              fontSize: '13px',
              fontWeight: '600',
              color: '#2563eb',
              textDecoration: 'none',
              whiteSpace: 'nowrap',
            }}
          >
            PR #{entry.pr_number}
          </a>
          <span
            style={{
              fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
              fontSize: '12px',
              color: '#475569',
              background: '#f1f5f9',
              border: '1px solid #e2e8f0',
              borderRadius: '4px',
              padding: '1px 7px',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              maxWidth: '280px',
            }}
            title={entry.branch_name}
          >
            {entry.branch_name}
          </span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0, flexWrap: 'wrap' }}>
          <StatusBadge status={entry.status} />
          <span style={{ fontSize: '11px', color: '#94a3b8', whiteSpace: 'nowrap' }}>
            synced {synced}
          </span>

          {!isMerged && onRefresh && (
            <>
              <button
                onClick={handleMarkMerged}
                disabled={marking}
                title="Mark as merged and retarget child PR"
                style={{
                  ...ghostBtnBase,
                  color: marking ? '#94a3b8' : '#475569',
                  borderColor: '#d1d5db',
                }}
              >
                {marking ? '…' : 'Merged'}
              </button>
              <button
                onClick={handleRemove}
                disabled={removing}
                title="Remove PR from this stack"
                style={{
                  ...ghostBtnBase,
                  color: removing ? '#94a3b8' : '#dc2626',
                  borderColor: removing ? '#d1d5db' : '#fca5a5',
                }}
              >
                {removing ? '…' : 'Remove'}
              </button>
            </>
          )}
        </div>
      </div>

      {entry.status === 'conflict' && (
        <ConflictNote
          branchName={entry.branch_name}
          parentBranchName={parentBranchName}
        />
      )}
    </div>
  );
}
