import React from 'react';
import Prism from '../../assets/js/prism';

type Props = {
  value: string;
  lineNumbers?: boolean;
  startLine?: number;
};

const language = 'yaml';

function DAGDefinition({ value, lineNumbers, startLine }: Props) {
  React.useEffect(() => {
    Prism.highlightAll();
  }, [value]);
  const className = React.useMemo(() => {
    if (lineNumbers) {
      return `language-${language} line-numbers`;
    }
    return `language-${language}`;
  }, [lineNumbers]);
  return (
    <pre
      style={{
        fontSize: '0.9rem',
      }}
      data-start={startLine || 1}
    >
      <code className={className}>{value}</code>
    </pre>
  );
}

export default DAGDefinition;
