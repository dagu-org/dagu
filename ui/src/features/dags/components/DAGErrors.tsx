import React from 'react';
import { components } from '../../../api/v2/schema';

type Props = {
  dags: components['schemas']['DAGFile'][];
  errors: string[];
  hasError: boolean;
};

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
              return dag.errors.map((err) => (
                <li key={err}>
                  {' '}
                  <a href={url}>{dag.dag.name}</a>: {err}{' '}
                </li>
              ));
            })}
          {errors.map((e) => (
            <li>{e}</li>
          ))}
        </ul>
      </div>
    </div>
  );
}
export default DAGErrors;
