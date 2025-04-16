/**
 * DAGPagination component provides pagination controls for DAG lists.
 *
 * @module features/dags/components/common
 */
import { Box, Pagination, TextField } from '@mui/material';
import React from 'react';

/**
 * Props for the DAGPagination component
 */
type DAGPaginationProps = {
  /** Total number of pages */
  totalPages: number;
  /** Current page number */
  page: number;
  /** Number of items per page */
  pageLimit: number;
  /** Callback for page change */
  pageChange: (page: number) => void;
  /** Callback for page limit change */
  onPageLimitChange: (pageLimit: number) => void;
};

/**
 * DAGPagination provides pagination controls with customizable page size
 */
const DAGPagination = ({
  totalPages,
  page,
  pageChange,
  pageLimit,
  onPageLimitChange,
}: DAGPaginationProps) => {
  // State for the input field value
  const [inputValue, setInputValue] = React.useState(pageLimit.toString());

  // Update input value when pageLimit prop changes
  React.useEffect(() => {
    setInputValue(pageLimit.toString());
  }, [pageLimit]);

  /**
   * Handle page change from pagination component
   */
  const handleChange = (event: React.ChangeEvent<unknown>, value: number) => {
    pageChange(value);
  };

  /**
   * Handle input change for page limit
   */
  const handleLimitChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const value = event.target.value;
    setInputValue(value);
  };

  /**
   * Apply the page limit change
   */
  const commitChange = () => {
    const numValue = parseInt(inputValue);
    if (!isNaN(numValue) && numValue > 0) {
      onPageLimitChange(numValue);
    } else {
      // Reset to current page limit if invalid input
      setInputValue(pageLimit.toString());
    }
  };

  /**
   * Handle Enter key press to commit changes
   */
  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') {
      commitChange();
      event.preventDefault();
      (event.target as HTMLInputElement).blur(); // Remove focus after Enter
    }
  };

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'row',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 3,
        mt: 2,
      }}
    >
      <TextField
        size="small"
        label="Items per page"
        value={inputValue}
        onChange={handleLimitChange}
        onBlur={commitChange}
        onKeyDown={handleKeyDown}
        inputProps={{
          type: 'number',
          min: '1',
          style: {
            width: '100px',
            textAlign: 'left',
          },
        }}
      />
      <Pagination count={totalPages} page={page} onChange={handleChange} />
    </Box>
  );
};

export default DAGPagination;
