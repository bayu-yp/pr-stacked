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

export default function PRNode({ entry, repoOwner, repoName, parentBranchName }) {
  const prUrl = `https://github.com/${repoOwner}/${repoName}/pull/${entry.pr_number}`;
  const synced = relativeTime(entry.last_synced);

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
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexShrink: 0 }}>
          <StatusBadge status={entry.status} />
          <span style={{ fontSize: '11px', color: '#94a3b8', whiteSpace: 'nowrap' }}>
            synced {synced}
          </span>
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
