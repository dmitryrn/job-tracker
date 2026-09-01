function valueLabel(value: unknown) {
  return JSON.stringify(value) ?? "undefined";
}

function JSONNode({ name, value, depth }: { name: string; value: unknown; depth: number }) {
  if (name === "bodyText" && typeof value === "string") {
    return (
      <details className="json-body">
        <summary><span className="json-key">&quot;{name}&quot;: </span><span className="json-string">{valueLabel(value)}</span></summary>
        <pre>{value}</pre>
      </details>
    );
  }

  if (Array.isArray(value)) {
    return (
      <details className="json-node" open={depth < 2}>
        <summary><span className="json-key">&quot;{name}&quot;: </span><span className="json-summary">Array({value.length})</span></summary>
        <div className="json-children">
          {value.map((item, index) => <JSONNode key={index} name={`[${index}]`} value={item} depth={depth + 1} />)}
        </div>
      </details>
    );
  }

  if (value !== null && typeof value === "object") {
    const entries = Object.entries(value);
    return (
      <details className="json-node" open={depth < 2}>
        <summary><span className="json-key">&quot;{name}&quot;: </span><span className="json-summary">Object({entries.length})</span></summary>
        <div className="json-children">
          {entries.map(([key, item]) => <JSONNode key={key} name={key} value={item} depth={depth + 1} />)}
        </div>
      </details>
    );
  }

  return <div className="json-property"><span className="json-key">&quot;{name}&quot;: </span><span className={`json-value json-${value === null ? "null" : typeof value}`}>{valueLabel(value)}</span></div>;
}

export default function JSONTree({ value }: { value: unknown }) {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    return <JSONNode name="value" value={value} depth={0} />;
  }

  return <div className="json-tree">{Object.entries(value as Record<string, unknown>).map(([key, item]) => <JSONNode key={key} name={key} value={item} depth={0} />)}</div>;
}
