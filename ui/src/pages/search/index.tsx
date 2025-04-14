import React, { useEffect, useRef } from 'react';
import { Box, Button, Grid, Stack, TextField, Typography } from '@mui/material';
import { useSearchParams } from 'react-router-dom';
import Title from '../../components/atoms/Title';
import SearchResult from '../../components/molecules/SearchResult';
import LoadingIndicator from '../../components/atoms/LoadingIndicator';
import { AppBarContext } from '../../contexts/AppBarContext';
import { useQuery } from '../../hooks/api';

function Search() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [searchVal, setSearchVal] = React.useState(searchParams.get('q') || '');
  const appBarContext = React.useContext(AppBarContext);

  const { data, isLoading } = useQuery(
    '/dags/search',
    {
      params: {
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
          q: searchParams.get('q') || '',
        },
      },
    },
    { refreshInterval: 2000 }
  );

  const ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    ref.current?.focus();
  }, [ref.current]);

  const onSubmit = React.useCallback((value: string) => {
    setSearchParams({
      q: value,
    });
  }, []);

  if (isLoading) {
    return <LoadingIndicator />;
  }

  return (
    <Grid container sx={{ mx: 4, width: '100%' }}>
      <Grid item xs={12}>
        <Title>Search</Title>
        <Stack spacing={2} direction="row">
          <TextField
            label="Search Text"
            variant="outlined"
            style={{
              flex: 0.5,
            }}
            inputRef={ref}
            InputProps={{
              value: searchVal,
              onChange: (e) => {
                setSearchVal(e.target.value);
              },
              type: 'search',
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                if (searchVal) {
                  onSubmit(searchVal);
                }
              }
            }}
          />
          <Button
            disabled={!searchVal}
            variant="outlined"
            sx={{
              width: '100px',
              border: 0,
            }}
            onClick={async () => {
              onSubmit(searchVal);
            }}
          >
            Search
          </Button>
        </Stack>

        <Box mt={2}>
          {(() => {
            if (data && data.results && data.results.length > 0) {
              return (
                <Box>
                  <Typography variant="h6" style={{ fontStyle: 'bolder' }}>
                    {data.results.length} results found
                  </Typography>
                  <SearchResult results={data.results} />
                </Box>
              );
            }

            if (
              (data && !data.results) ||
              (data && data.results && data.results.length === 0)
            ) {
              return <Box>No results found</Box>;
            }

            return null;
          })()}
        </Box>
      </Grid>
    </Grid>
  );
}
export default Search;
