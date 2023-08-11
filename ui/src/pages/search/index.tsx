import React, { useEffect, useRef } from 'react';
import { Box, Button, Grid, Stack, TextField, Typography } from '@mui/material';
import useSWR from 'swr';
import { useSearchParams } from 'react-router-dom';
import Title from '../../components/atoms/Title';
import { GetSearchResponse } from '../../models/api';
import SearchResult from '../../components/molecules/SearchResult';
import LoadingIndicator from '../../components/atoms/LoadingIndicator';

function Search() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [searchVal, setSearchVal] = React.useState(searchParams.get('q') || '');

  const { data, error } = useSWR<GetSearchResponse>(
    `/search?q=${searchParams.get('q') || ''}`
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
          {!data && !error ? <LoadingIndicator /> : null}
          {data && data.Results.length == 0 ? (
            <Box>No results found</Box>
          ) : null}
          {data && data.Results.length > 0 ? (
            <Box>
              <Typography variant="h6" style={{ fontStyle: 'bolder' }}>
                {data.Results.length} results found
              </Typography>
              <SearchResult results={data?.Results} />
            </Box>
          ) : null}
        </Box>
      </Grid>
    </Grid>
  );
}
export default Search;
