import React from 'react';
import Prism from '../../assets/js/prism';

type Props = {
  value: string;
  lineNumbers?: boolean;
  highlightLine?: number;
  startLine?: number;
  keyword?: string;
  noHighlight?: boolean;
};

const language = 'yaml';

function DAGDefinition({
  value,
  lineNumbers,
  highlightLine,
  startLine,
  keyword,
  noHighlight,
}: Props) {
  React.useEffect(() => {
    if (!noHighlight) {
      Prism.highlightAll();
    }
  }, [value]);
  const classes = [`language-${language}`];
  if (lineNumbers) {
    classes.push('line-numbers');
  }
  if (keyword) {
    classes.push(`keyword-${keyword}`);
  }
  const className = classes.join(' ');
  return (
    <pre data-start={startLine || 1} data-line={highlightLine}>
      <code className={className}>{value}</code>
    </pre>
  );
}

export default DAGDefinition;
