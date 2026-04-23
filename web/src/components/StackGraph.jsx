import { useState } from 'react';
import PRNode from './PRNode.jsx';
import SyncButton from './SyncButton.jsx';
import AddPRForm from './AddPRForm.jsx';

export default function StackGraph({ stack, entries, apiUrl, onSyncComplete, onRefresh }) {
  const [showAddPR, setShowAddPR] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const sorted = [...entries].sort((a, b) => a.position - b.position);

  async function handleDelete() {
    if (!window.confirm(`Delete stack "${stack.name}"? This will remove all PR entries. The PRs themselves on GitHub are not affected.`)) return;
    setDeleting(true);
    try {
      await fetch(`${apiUrl}/api/stacks/${stack.id}`, { method: 'DELETE' });
      onRefresh();
    } finally {
      setDeleting(false);
    }
  }

  const ghostBtn = {
    padding: '5px 12px',
    fontSize: '12px',
    fontWeight: '500',
    borderRadius: '6px',
    border: '1px solid',
    cursor: 'pointer',
    background: 'transparent',
    lineHeight: '1.4',
  };

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
          flexWrap: 'wrap',
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

        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexShrink: 0 }}>
          <button
            onClick={() => setShowAddPR((v) => !v)}
            style={{
              ...ghostBtn,
              color: showAddPR ? '#374151' : '#2563eb',
              borderColor: showAddPR ? '#d1d5db' : '#93c5fd',
            }}
          >
            {showAddPR ? 'Cancel' : '+ Add PR'}
          </button>

          <SyncButton
            stackID={stack.id}
            apiUrl={apiUrl}
            onSyncComplete={onSyncComplete}
          />

          <button
            onClick={handleDelete}
            disabled={deleting}
            title="Delete this stack"
            style={{
              ...ghostBtn,
              color: deleting ? '#94a3b8' : '#dc2626',
              borderColor: deleting ? '#d1d5db' : '#fca5a5',
            }}
          >
            {deleting ? '…' : 'Delete'}
          </button>
        </div>
      </div>

      {/* Chain body */}
      <div style={{ padding: '20px 20px 16px' }}>
        {showAddPR && (
          <AddPRForm
            stackID={stack.id}
            apiUrl={apiUrl}
            onAdded={() => { setShowAddPR(false); onRefresh(); }}
            onCancel={() => setShowAddPR(false)}
          />
        )}

        {sorted.length === 0 ? (
          <p style={{ margin: showAddPR ? '12px 0 0' : '0', fontSize: '13px', color: '#94a3b8', textAlign: 'center', padding: '12px 0' }}>
            No entries in this stack yet.
          </p>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', marginTop: showAddPR ? '12px' : '0' }}>
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
                    stackID={stack.id}
                    apiUrl={apiUrl}
                    onRefresh={onRefresh}
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
