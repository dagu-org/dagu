/**
 * DAGErrors component displays a list of errors for DAGs.
 *
 * @module features/dags/components/dag-editor
 */
import React from 'react';
import { components } from '../../../../api/v2/schema';

/**
 * Props for the DAGErrors component
 */
type Props = {
  /** List of DAG files */
  dags: components['schemas']['DAGFile'][];
  /** List of general errors */
  errors: string[];
  /** Whether there are any errors */
  hasError: boolean;
};

/**
 * DAGErrors displays a list of errors for DAGs
 * with links to the corresponding DAG pages
 */
function DAGErrors({ dags, errors, hasError }: Props) {
  if (!dags || !hasError) {
    return <div></div>;
  }

  return (
    <div className="notification is-danger mt-0 mb-0">
      <div>Please check the below errors!</div>
      <div className="content">
        <ul>
          {dags
            .filter((dag) => dag.errors.length > 0)
            .map((dag) => {
              const url = encodeURI(dag.dag.name);
              return dag.errors.map((err, index) => (
                <li key={`${dag.dag.name}-${index}`}>
                  <a href={url}>{dag.dag.name}</a>: {err}
                </li>
              ));
            })}
          {errors.map((e, index) => (
            <li key={`general-${index}`}>{e}</li>
          ))}
        </ul>
      </div>
    </div>
  );
}

export default DAGErrors;
