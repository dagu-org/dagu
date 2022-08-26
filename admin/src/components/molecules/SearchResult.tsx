import { Divider, List, ListItem, Stack, Typography } from '@mui/material';
import React, { ReactElement, useEffect } from 'react';
import { Link } from 'react-router-dom';
import { SearchResult } from '../../models/api';
import DAGDefinition from './DAGDefinition';
import Prism from '../../assets/js/prism';

type Props = {
  results: SearchResult[];
};

function SearchResult({ results }: Props) {
  const elements = React.useMemo(
    () =>
      results.map((result, i) => {
        const ret = [] as ReactElement[];
        result.Matches.forEach((m, j) => {
          ret.push(
            <ListItem key={`${result.Name}-${m.LineNumber}`}>
              <Stack direction="column" spacing={1} style={{ width: '100%' }}>
                {j == 0 ? (
                  <Link to={`/dags/${encodeURI(result.Name)}/spec`}>
                    <Typography variant="h6">{result.Name}</Typography>
                  </Link>
                ) : null}
                <DAGDefinition
                  value={m.Line}
                  lineNumbers
                  startLine={m.StartLine}
                  highlightLine={m.LineNumber - m.StartLine}
                  noHighlight
                />
              </Stack>
            </ListItem>
          );
        });
        if (i < results.length - 1) {
          ret.push(<Divider key={`${result.Name}-divider`} />);
        }
        return ret;
      }),
    [results]
  );
  useEffect(() => Prism.highlightAll(), [elements]);
  return <List>{elements}</List>;
}
export default SearchResult;
