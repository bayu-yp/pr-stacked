export default function ConflictNote({ branchName, parentBranchName }) {
  return (
    <div
      style={{
        background: '#fef9c3',
        border: '1px solid #fbbf24',
        borderRadius: '6px',
        padding: '10px 14px',
        marginTop: '8px',
        fontSize: '13px',
        color: '#78350f',
      }}
    >
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
          fontWeight: '600',
          marginBottom: '6px',
        }}
      >
        <span aria-hidden="true" style={{ fontSize: '14px' }}>&#9888;</span>
        Merge conflict — resolve locally:
      </div>
      <pre
        style={{
          margin: '0',
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
          fontSize: '12px',
          lineHeight: '1.6',
          color: '#451a03',
          whiteSpace: 'pre',
          overflowX: 'auto',
        }}
      >{`git checkout ${branchName}
git merge ${parentBranchName}
# resolve conflicts
git push`}</pre>
    </div>
  );
}
