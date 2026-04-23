import { useState, useEffect, useCallback } from 'react';
import StackGraph from './StackGraph.jsx';
import CreateStackModal from './CreateStackModal.jsx';

function Spinner() {
  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: '12px',
        padding: '64px 0',
        color: '#64748b',
      }}
    >
      <span
        style={{
          display: 'inline-block',
          width: '28px',
          height: '28px',
          border: '3px solid #e2e8f0',
          borderTopColor: '#3b82f6',
          borderRadius: '50%',
          animation: 'spin 0.7s linear infinite',
        }}
        role="status"
        aria-label="Loading"
      />
      <span style={{ fontSize: '14px' }}>Loading stacks...</span>
      <style>{`
        @keyframes spin {
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}

export default function StackList({ apiUrl }) {
  const [stacks, setStacks] = useState([]);
  const [entriesByStack, setEntriesByStack] = useState({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showCreateModal, setShowCreateModal] = useState(false);

  const fetchAll = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const stacksRes = await fetch(`${apiUrl}/api/stacks`);
      if (!stacksRes.ok) {
        throw new Error(`Failed to load stacks: HTTP ${stacksRes.status}`);
      }
      const stacksData = await stacksRes.json();

      const entriesResults = await Promise.allSettled(
        stacksData.map((stack) =>
          fetch(`${apiUrl}/api/stacks/${stack.id}/entries`).then((r) => {
            if (!r.ok) throw new Error(`HTTP ${r.status}`);
            return r.json();
          })
        )
      );

      const entriesMap = {};
      stacksData.forEach((stack, idx) => {
        const result = entriesResults[idx];
        entriesMap[stack.id] = result.status === 'fulfilled' ? result.value : [];
      });

      setStacks(stacksData);
      setEntriesByStack(entriesMap);
    } catch (err) {
      setError(err.message ?? 'An unexpected error occurred.');
    } finally {
      setLoading(false);
    }
  }, [apiUrl]);

  useEffect(() => {
    fetchAll();
  }, [fetchAll]);

  return (
    <div>
      {/* Toolbar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: '24px',
          gap: '10px',
          flexWrap: 'wrap',
        }}
      >
        <h2
          style={{
            margin: '0',
            fontSize: '20px',
            fontWeight: '700',
            color: '#0f172a',
            letterSpacing: '-0.3px',
          }}
        >
          PR Stacks
          {!loading && !error && (
            <span
              style={{
                marginLeft: '8px',
                fontSize: '13px',
                fontWeight: '500',
                color: '#64748b',
                background: '#f1f5f9',
                border: '1px solid #e2e8f0',
                borderRadius: '9999px',
                padding: '1px 8px',
                verticalAlign: 'middle',
              }}
            >
              {stacks.length}
            </span>
          )}
        </h2>

        <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
          <button
            onClick={() => setShowCreateModal(true)}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '6px',
              padding: '6px 14px',
              background: '#3b82f6',
              color: '#ffffff',
              border: 'none',
              borderRadius: '6px',
              fontSize: '13px',
              fontWeight: '500',
              cursor: 'pointer',
              boxShadow: '0 1px 2px rgba(0,0,0,0.05)',
            }}
          >
            + New Stack
          </button>

          <button
            onClick={fetchAll}
            disabled={loading}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '6px',
              padding: '6px 14px',
              background: '#ffffff',
              color: loading ? '#94a3b8' : '#374151',
              border: '1px solid #d1d5db',
              borderRadius: '6px',
              fontSize: '13px',
              fontWeight: '500',
              cursor: loading ? 'not-allowed' : 'pointer',
              boxShadow: '0 1px 2px rgba(0,0,0,0.05)',
            }}
          >
            <svg
              width="13"
              height="13"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
              style={{ opacity: loading ? 0.5 : 1 }}
            >
              <polyline points="1 4 1 10 7 10" />
              <polyline points="23 20 23 14 17 14" />
              <path d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4-4.64 4.36A9 9 0 0 1 3.51 15" />
            </svg>
            Refresh
          </button>
        </div>
      </div>

      {/* States */}
      {loading && <Spinner />}

      {!loading && error && (
        <div
          role="alert"
          style={{
            background: '#fef2f2',
            border: '1px solid #fecaca',
            borderRadius: '8px',
            padding: '16px 20px',
            color: '#991b1b',
            fontSize: '14px',
            display: 'flex',
            alignItems: 'flex-start',
            gap: '10px',
          }}
        >
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            style={{ flexShrink: 0, marginTop: '1px' }}
            aria-hidden="true"
          >
            <circle cx="12" cy="12" r="10" />
            <line x1="12" y1="8" x2="12" y2="12" />
            <line x1="12" y1="16" x2="12.01" y2="16" />
          </svg>
          <div>
            <strong style={{ display: 'block', marginBottom: '2px' }}>Failed to load stacks</strong>
            {error}
          </div>
        </div>
      )}

      {!loading && !error && stacks.length === 0 && (
        <div
          style={{
            textAlign: 'center',
            padding: '64px 0',
            color: '#94a3b8',
          }}
        >
          <svg
            width="40"
            height="40"
            viewBox="0 0 24 24"
            fill="none"
            stroke="#cbd5e1"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            style={{ display: 'block', margin: '0 auto 12px' }}
            aria-hidden="true"
          >
            <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
            <line x1="3" y1="9" x2="21" y2="9" />
            <line x1="3" y1="15" x2="21" y2="15" />
            <line x1="9" y1="3" x2="9" y2="21" />
          </svg>
          <p style={{ margin: '0 0 4px', fontSize: '15px', fontWeight: '500', color: '#64748b' }}>
            No stacks found
          </p>
          <p style={{ margin: '0 0 16px', fontSize: '13px' }}>
            Create your first stack to get started.
          </p>
          <button
            onClick={() => setShowCreateModal(true)}
            style={{
              padding: '8px 18px',
              background: '#3b82f6',
              color: '#fff',
              border: 'none',
              borderRadius: '6px',
              fontSize: '13px',
              fontWeight: '500',
              cursor: 'pointer',
            }}
          >
            Create Stack
          </button>
        </div>
      )}

      {!loading && !error && stacks.length > 0 && (
        <div>
          {Object.entries(
            stacks.reduce((groups, stack) => {
              const key = `${stack.repo_owner}/${stack.repo_name}`;
              if (!groups[key]) groups[key] = [];
              groups[key].push(stack);
              return groups;
            }, {})
          ).map(([repoKey, repoStacks]) => (
            <div key={repoKey} style={{ marginBottom: '40px' }}>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                  marginBottom: '16px',
                  paddingBottom: '10px',
                  borderBottom: '1px solid #e2e8f0',
                }}
              >
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#64748b" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                  <path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22" />
                </svg>
                <span style={{ fontSize: '13px', fontWeight: '600', color: '#475569', fontFamily: 'ui-monospace, monospace' }}>
                  {repoKey}
                </span>
                <span style={{ fontSize: '12px', color: '#94a3b8' }}>
                  {repoStacks.length} {repoStacks.length === 1 ? 'stack' : 'stacks'}
                </span>
              </div>
              {repoStacks.map((stack) => (
                <StackGraph
                  key={stack.id}
                  stack={stack}
                  entries={entriesByStack[stack.id] ?? []}
                  apiUrl={apiUrl}
                  onSyncComplete={fetchAll}
                  onRefresh={fetchAll}
                />
              ))}
            </div>
          ))}
        </div>
      )}

      {showCreateModal && (
        <CreateStackModal
          apiUrl={apiUrl}
          onCreated={() => { setShowCreateModal(false); fetchAll(); }}
          onClose={() => setShowCreateModal(false)}
        />
      )}
    </div>
  );
}
