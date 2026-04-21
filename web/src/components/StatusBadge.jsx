const STATUS_STYLES = {
  synced: {
    background: '#dcfce7',
    color: '#15803d',
    border: '1px solid #86efac',
  },
  conflict: {
    background: '#fee2e2',
    color: '#b91c1c',
    border: '1px solid #fca5a5',
  },
  pending: {
    background: '#fef3c7',
    color: '#92400e',
    border: '1px solid #fcd34d',
  },
  merged: {
    background: '#f1f5f9',
    color: '#475569',
    border: '1px solid #cbd5e1',
  },
};

const STATUS_DOTS = {
  synced: '#22c55e',
  conflict: '#ef4444',
  pending: '#f59e0b',
  merged: '#6b7280',
};

export default function StatusBadge({ status }) {
  const style = STATUS_STYLES[status] ?? STATUS_STYLES.pending;
  const dotColor = STATUS_DOTS[status] ?? STATUS_DOTS.pending;

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '5px',
        padding: '2px 9px',
        borderRadius: '9999px',
        fontSize: '12px',
        fontWeight: '500',
        whiteSpace: 'nowrap',
        ...style,
      }}
    >
      <span
        style={{
          width: '6px',
          height: '6px',
          borderRadius: '50%',
          background: dotColor,
          flexShrink: 0,
        }}
      />
      {status ? status.charAt(0).toUpperCase() + status.slice(1) : 'Unknown'}
    </span>
  );
}
