import { useState } from 'react';

export default function AddPRForm({ stackID, apiUrl, onAdded, onCancel }) {
  const [prNumber, setPrNumber] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const parsed = parseInt(prNumber, 10);
  const canSubmit = !isNaN(parsed) && parsed > 0 && !loading;

  async function handleSubmit(e) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`${apiUrl}/api/stacks/${stackID}/entries`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ pr_number: parsed }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `HTTP ${res.status}`);
        return;
      }
      onAdded();
    } catch (err) {
      setError(err.message ?? 'Unexpected error');
    } finally {
      setLoading(false);
    }
  }

  return (
    <form
      onSubmit={handleSubmit}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        padding: '10px 12px',
        background: '#f8fafc',
        border: '1px solid #e2e8f0',
        borderRadius: '6px',
        marginTop: '8px',
        flexWrap: 'wrap',
      }}
    >
      <input
        type="number"
        min="1"
        placeholder="PR number"
        value={prNumber}
        onChange={(e) => setPrNumber(e.target.value)}
        autoFocus
        style={{
          width: '120px',
          padding: '6px 10px',
          fontSize: '13px',
          border: '1px solid #d1d5db',
          borderRadius: '6px',
          outline: 'none',
          fontFamily: 'inherit',
        }}
      />

      <button
        type="submit"
        disabled={!canSubmit}
        style={{
          padding: '6px 14px',
          background: canSubmit ? '#3b82f6' : '#93c5fd',
          color: '#fff',
          border: 'none',
          borderRadius: '6px',
          fontSize: '13px',
          fontWeight: '500',
          cursor: canSubmit ? 'pointer' : 'not-allowed',
        }}
      >
        {loading ? 'Adding…' : 'Add PR'}
      </button>

      <button
        type="button"
        onClick={onCancel}
        style={{
          padding: '6px 10px',
          background: 'transparent',
          color: '#64748b',
          border: 'none',
          fontSize: '13px',
          cursor: 'pointer',
        }}
      >
        Cancel
      </button>

      {error && (
        <span
          role="alert"
          style={{ fontSize: '12px', color: '#dc2626', width: '100%', marginTop: '2px' }}
        >
          {error}
        </span>
      )}
    </form>
  );
}
