import PRNode from './PRNode.jsx';
import SyncButton from './SyncButton.jsx';

export default function StackGraph({ stack, entries, apiUrl, onSyncComplete }) {
  const sorted = [...entries].sort((a, b) => a.position - b.position);

  return (
    <div
      style={{
        background: '#ffffff',
        border: '1px solid #e2e8f0',
        borderRadius: '10px',
        boxShadow: '0 1px 4px rgba(0,0,0,0.07)',
        overflow: 'hidden',
        marginBottom: '24px',
      }}
    >
      {/* Stack header */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '14px 20px',
          borderBottom: '1px solid #e2e8f0',
          background: '#f8fafc',
          gap: '12px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', minWidth: 0 }}>
          <span
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: '22px',
              height: '22px',
              background: '#dbeafe',
              borderRadius: '4px',
              flexShrink: 0,
            }}
            aria-hidden="true"
          >
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#2563eb" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
              <line x1="6" y1="3" x2="6" y2="15" />
              <circle cx="18" cy="6" r="3" />
              <circle cx="6" cy="18" r="3" />
              <path d="M18 9a9 9 0 0 1-9 9" />
            </svg>
          </span>
          <h2
            style={{
              margin: '0',
              fontSize: '15px',
              fontWeight: '600',
              color: '#0f172a',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {stack.name}
          </h2>
          <span
            style={{
              fontSize: '12px',
              color: '#64748b',
              background: '#f1f5f9',
              border: '1px solid #e2e8f0',
              borderRadius: '4px',
              padding: '1px 7px',
              whiteSpace: 'nowrap',
              fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
            }}
          >
            {stack.repo_owner}/{stack.repo_name}
          </span>
        </div>
        <SyncButton
          stackID={stack.id}
          apiUrl={apiUrl}
          onSyncComplete={onSyncComplete}
        />
      </div>

      {/* Chain body */}
      <div style={{ padding: '20px 20px 16px' }}>
        {sorted.length === 0 ? (
          <p style={{ margin: '0', fontSize: '13px', color: '#94a3b8', textAlign: 'center', padding: '12px 0' }}>
            No entries in this stack yet.
          </p>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column' }}>
            {/* Base indicator */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '8px',
                marginBottom: '6px',
                paddingLeft: '20px',
              }}
            >
              <span
                style={{
                  fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                  fontSize: '12px',
                  color: '#64748b',
                }}
              >
                base: main
              </span>
              <svg width="10" height="10" viewBox="0 0 10 10" aria-hidden="true">
                <polyline points="5,1 5,9" stroke="#94a3b8" strokeWidth="1.5" strokeDasharray="2,2" strokeLinecap="round" />
                <polyline points="2,4 5,1 8,4" stroke="#94a3b8" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" fill="none" />
              </svg>
            </div>

            {sorted.map((entry, idx) => {
              const parentBranchName =
                idx === 0 ? 'main' : sorted[idx - 1].branch_name;

              return (
                <div key={entry.id} style={{ display: 'flex', flexDirection: 'column' }}>
                  {/* Connector line above each node (skip for first) */}
                  {idx > 0 && (
                    <div
                      style={{
                        alignSelf: 'flex-start',
                        marginLeft: '20px',
                        width: '1px',
                        height: '20px',
                        borderLeft: '2px dashed #cbd5e1',
                      }}
                      aria-hidden="true"
                    />
                  )}
                  <PRNode
                    entry={entry}
                    repoOwner={stack.repo_owner}
                    repoName={stack.repo_name}
                    parentBranchName={parentBranchName}
                  />
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
