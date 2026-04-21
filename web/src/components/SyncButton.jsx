import { useState } from 'react';

export default function SyncButton({ stackID, apiUrl, onSyncComplete }) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  async function handleSync() {
    setLoading(true);
    setError(null);

    try {
      const res = await fetch(`${apiUrl}/api/stacks/${stackID}/sync`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      });

      const data = await res.json();

      if (!res.ok || data.ok === false) {
        throw new Error(data.error ?? `HTTP ${res.status}`);
      }

      onSyncComplete();
    } catch (err) {
      setError(err.message ?? 'Sync failed');
      setTimeout(() => setError(null), 5000);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: '4px' }}>
      <button
        onClick={handleSync}
        disabled={loading}
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: '6px',
          padding: '5px 12px',
          background: loading ? '#e2e8f0' : '#3b82f6',
          color: loading ? '#94a3b8' : '#ffffff',
          border: 'none',
          borderRadius: '6px',
          fontSize: '13px',
          fontWeight: '500',
          cursor: loading ? 'not-allowed' : 'pointer',
          transition: 'background 0.15s ease',
          whiteSpace: 'nowrap',
        }}
      >
        {loading ? (
          <>
            <span
              style={{
                display: 'inline-block',
                width: '12px',
                height: '12px',
                border: '2px solid #94a3b8',
                borderTopColor: 'transparent',
                borderRadius: '50%',
                animation: 'spin 0.6s linear infinite',
              }}
            />
            Syncing...
          </>
        ) : (
          <>
            <svg
              width="12"
              height="12"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              aria-hidden="true"
            >
              <polyline points="1 4 1 10 7 10" />
              <polyline points="23 20 23 14 17 14" />
              <path d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4-4.64 4.36A9 9 0 0 1 3.51 15" />
            </svg>
            Sync All
          </>
        )}
      </button>
      {error && (
        <span
          role="alert"
          style={{
            fontSize: '12px',
            color: '#b91c1c',
            background: '#fee2e2',
            border: '1px solid #fca5a5',
            borderRadius: '4px',
            padding: '2px 8px',
          }}
        >
          {error}
        </span>
      )}
      <style>{`
        @keyframes spin {
          to { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}
