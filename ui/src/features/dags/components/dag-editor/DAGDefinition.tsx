/**
 * DAGDefinition component displays a syntax-highlighted YAML definition.
 *
 * @module features/dags/components/dag-editor
 */
import React from 'react';
import Prism from '../../../../assets/js/prism';

/**
 * Props for the DAGDefinition component
 */
type Props = {
  /** YAML content to display */
  value: string;
  /** Whether to show line numbers */
  lineNumbers?: boolean;
  /** Line number to highlight */
  highlightLine?: number;
  /** Starting line number */
  startLine?: number;
  /** Keyword to highlight */
  keyword?: string;
  /** Whether to disable syntax highlighting */
  noHighlight?: boolean;
};

/** Language for syntax highlighting */
const language = 'yaml';

/**
 * DAGDefinition displays a syntax-highlighted YAML definition
 * using Prism.js for syntax highlighting
 */
function DAGDefinition({
  value,
  lineNumbers,
  highlightLine,
  startLine,
  keyword,
  noHighlight,
}: Props) {
  // Apply syntax highlighting when the component mounts or value changes
  React.useEffect(() => {
    if (!noHighlight) {
      Prism.highlightAll();
    }
  }, [value, noHighlight]);

  // Build class names for Prism
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
