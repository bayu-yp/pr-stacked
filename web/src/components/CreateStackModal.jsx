import { useState, useEffect } from 'react';

export default function CreateStackModal({ apiUrl, onCreated, onClose }) {
  const [name, setName] = useState('');
  const [repoOwner, setRepoOwner] = useState('');
  const [repoName, setRepoName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  useEffect(() => {
    const onKey = (e) => { if (e.key === 'Escape') onClose(); };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [onClose]);

  const canSubmit = name.trim() && repoOwner.trim() && repoName.trim() && !loading;

  async function handleSubmit(e) {
    e.preventDefault();
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`${apiUrl}/api/stacks`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: name.trim(), repo_owner: repoOwner.trim(), repo_name: repoName.trim() }),
      });
      const data = await res.json();
      if (!res.ok) {
        setError(data.error ?? `HTTP ${res.status}`);
        return;
      }
      onCreated();
    } catch (err) {
      setError(err.message ?? 'Unexpected error');
    } finally {
      setLoading(false);
    }
  }

  const inputStyle = {
    width: '100%',
    padding: '8px 10px',
    fontSize: '14px',
    border: '1px solid #d1d5db',
    borderRadius: '6px',
    outline: 'none',
    boxSizing: 'border-box',
    fontFamily: 'inherit',
  };

  const labelStyle = {
    display: 'block',
    fontSize: '13px',
    fontWeight: '500',
    color: '#374151',
    marginBottom: '4px',
  };

  return (
    <div
      onClick={onClose}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.4)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 100,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: '#fff',
          borderRadius: '10px',
          boxShadow: '0 20px 60px rgba(0,0,0,0.2)',
          width: '100%',
          maxWidth: '440px',
          padding: '28px',
        }}
      >
        <h2 style={{ margin: '0 0 20px', fontSize: '17px', fontWeight: '700', color: '#0f172a' }}>
          Create Stack
        </h2>

        <form onSubmit={handleSubmit}>
          <div style={{ marginBottom: '14px' }}>
            <label style={labelStyle}>Stack Name</label>
            <input
              type="text"
              placeholder="auth-feature"
              value={name}
              onChange={(e) => setName(e.target.value)}
              style={inputStyle}
              autoFocus
            />
          </div>

          <div style={{ marginBottom: '14px' }}>
            <label style={labelStyle}>Repo Owner</label>
            <input
              type="text"
              placeholder="myorg"
              value={repoOwner}
              onChange={(e) => setRepoOwner(e.target.value)}
              style={inputStyle}
            />
          </div>

          <div style={{ marginBottom: '20px' }}>
            <label style={labelStyle}>Repo Name</label>
            <input
              type="text"
              placeholder="myrepo"
              value={repoName}
              onChange={(e) => setRepoName(e.target.value)}
              style={inputStyle}
            />
          </div>

          {error && (
            <div
              role="alert"
              style={{
                marginBottom: '16px',
                padding: '10px 12px',
                background: '#fef2f2',
                border: '1px solid #fecaca',
                borderRadius: '6px',
                fontSize: '13px',
                color: '#991b1b',
              }}
            >
              {error}
            </div>
          )}

          <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
            <button
              type="button"
              onClick={onClose}
              style={{
                padding: '8px 16px',
                background: '#fff',
                color: '#374151',
                border: '1px solid #d1d5db',
                borderRadius: '6px',
                fontSize: '14px',
                fontWeight: '500',
                cursor: 'pointer',
              }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!canSubmit}
              style={{
                padding: '8px 16px',
                background: canSubmit ? '#3b82f6' : '#93c5fd',
                color: '#fff',
                border: 'none',
                borderRadius: '6px',
                fontSize: '14px',
                fontWeight: '500',
                cursor: canSubmit ? 'pointer' : 'not-allowed',
              }}
            >
              {loading ? 'Creating…' : 'Create Stack'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
