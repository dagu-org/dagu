import React, { ReactElement, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { components } from '../../../api/v1/schema';
import Prism from '../../../assets/js/prism';
import { DAGDefinition } from '../../dags/components/dag-editor';

type DagResult = components['schemas']['SearchResultItem'];
type DocResult = components['schemas']['DocSearchResultItem'];

type Props =
  | { type: 'dag'; results: DagResult[] }
  | { type: 'doc'; results: DocResult[] };

function SearchResult(props: Props) {
  const { type, results } = props;

  const elements = React.useMemo(
    () =>
      results.map((result, i) => {
        const name =
          type === 'dag'
            ? (result as DagResult).name
            : (result as DocResult).title;
        const link =
          type === 'dag'
            ? `/dags/${encodeURI((result as DagResult).name)}/spec`
            : `/docs/${(result as DocResult).id}`;
        const matches =
          type === 'dag'
            ? (result as DagResult).matches
            : ((result as DocResult).matches ?? []);

        const ret = [] as ReactElement[];
        matches.forEach((m, j) => {
          ret.push(
            <li key={`${name}-${m.lineNumber}`} className="px-4">
              <div className="flex flex-col space-y-2 w-full">
                {j == 0 ? (
                  <Link to={link}>
                    <h3 className="text-lg font-semibold text-foreground">
                      {name}
                      <span className="ml-2 text-xs font-normal text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
                        {type === 'dag' ? 'DAG' : 'Doc'}
                      </span>
                    </h3>
                  </Link>
                ) : null}
                <DAGDefinition
                  value={m.line}
                  lineNumbers
                  startLine={m.startLine}
                  highlightLine={m.lineNumber - m.startLine}
                  noHighlight
                />
              </div>
            </li>
          );
        });
        if (i < results.length - 1) {
          ret.push(
            <div
              key={`${name}-divider`}
              className="h-px bg-accent my-2"
            />
          );
        }
        return ret;
      }),
    [results, type]
  );

  useEffect(() => Prism.highlightAll(), [elements]);

  return <ul className="rounded-md border">{elements}</ul>;
}

export default SearchResult;
