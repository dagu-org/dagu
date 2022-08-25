import React from 'react';
import Prism from '../../assets/js/prism';

type Props = {
  value: string;
  lineNumbers?: boolean;
  startLine?: number;
  keyword?: string;
  noHighlight?: boolean;
};

const language = 'yaml';

function DAGDefinition({
  value,
  lineNumbers,
  startLine,
  keyword,
  noHighlight,
}: Props) {
  React.useEffect(() => {
    if (!noHighlight) {
      Prism.highlightAll();
    }
  }, [value]);
  const className = React.useMemo(() => {
    const classes = [`language-${language}`];
    if (lineNumbers) {
      classes.push('line-numbers');
    }
    if (keyword) {
      classes.push(`keyword-${keyword}`);
    }
    return classes.join(' ');
  }, [lineNumbers, keyword]);
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
