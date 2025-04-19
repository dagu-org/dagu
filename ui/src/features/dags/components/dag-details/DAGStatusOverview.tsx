/**
 * DAGStatusOverview component displays summary information about a DAG run.
 *
 * @module features/dags/components/dag-details
 */
import React from 'react';
import StatusChip from '../../../../ui/StatusChip';
import { Stack, Paper, Box, Divider, Typography } from '@mui/material';
import LabeledItem from '../../../../ui/LabeledItem';
import { Link } from 'react-router-dom';
import { DescriptionOutlined } from '@mui/icons-material';
import { components } from '../../../../api/v2/schema';

/**
 * Props for the DAGStatusOverview component
 */
type Props = {
  /** DAG run details */
  status?: components['schemas']['RunDetails'];
  /** DAG file ID */
  fileId: string;
  /** Request ID of the execution */
  requestId?: string;
};

/**
 * DAGStatusOverview displays summary information about a DAG run
 * including status, request ID, timestamps, and parameters
 */
function DAGStatusOverview({ status, fileId, requestId = '' }: Props) {
  // Build URL for log viewing
  const searchParams = new URLSearchParams();
  if (requestId) {
    searchParams.set('requestId', requestId);
  }
  const url = `/dags/${fileId}/scheduler-log?${searchParams.toString()}`;

  // Don't render if no status is provided
  if (!status) {
    return null;
  }

  // Format timestamps for better readability if they exist
  const formatTimestamp = (timestamp: string | undefined) => {
    if (!timestamp || timestamp == '-') return '-';
    try {
      const date = new Date(timestamp);
      return date.toLocaleString();
    } catch (e) {
      return timestamp;
    }
  };

  return (
    <Paper
      elevation={0}
      sx={{
        p: 2,
        borderRadius: 2,
        border: '1px solid rgba(0, 0, 0, 0.12)',
        transition: 'all 0.2s ease-in-out',
        '&:hover': {
          boxShadow: '0px 4px 8px rgba(0, 0, 0, 0.05)',
        },
      }}
    >
      <Box sx={{ mb: 2 }}>
        <Stack
          direction="row"
          spacing={2}
          sx={{ alignItems: 'center', justifyContent: 'space-between' }}
        >
          <StatusChip status={status.status} size="lg">
            {status.statusText}
          </StatusChip>
        </Stack>
      </Box>

      <Divider sx={{ my: 1.5 }} />

      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' },
          gap: 2,
        }}
      >
        {status.requestId && (
          <LabeledItem label="Request ID">
            <Box
              sx={{
                p: 1,
                bgcolor: 'rgba(0, 0, 0, 0.04)',
                borderRadius: 1,
                fontFamily: 'inherit',
                fontWeight: 500,
                fontSize: '0.875rem',
              }}
            >
              {status.requestId}
            </Box>
            <Box sx={{ display: 'flex', alignItems: 'center' }}>
              <Link
                to={url}
                style={{
                  textDecoration: 'none',
                  display: 'inline-flex',
                  alignItems: 'center',
                  padding: '6px 12px',
                  borderRadius: '4px',
                  color: 'rgba(0, 0, 0, 0.7)',
                  transition: 'all 0.2s ease',
                  fontWeight: 500,
                }}
              >
                <Box
                  component="span"
                  sx={{
                    mr: 1,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                >
                  <DescriptionOutlined fontSize="small" />
                </Box>
              </Link>
            </Box>
          </LabeledItem>
        )}
      </Box>

      <Box
        sx={{
          mt: 2,
          display: 'grid',
          gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' },
          gap: 2,
        }}
      >
        <LabeledItem label="Started At">
          <Box sx={{ fontFamily: 'inherit', fontWeight: 500 }}>
            {formatTimestamp(status.startedAt)}
          </Box>
        </LabeledItem>

        <LabeledItem label="Finished At">
          <Box sx={{ fontFamily: 'inherit', fontWeight: 500 }}>
            {formatTimestamp(status.finishedAt)}
          </Box>
        </LabeledItem>
      </Box>

      {status.params && (
        <Box sx={{ mt: 2 }}>
          <LabeledItem label="Parameters">
            <Box
              sx={{
                p: 1.5,
                bgcolor: 'rgba(0, 0, 0, 0.04)',
                borderRadius: 1,
                fontFamily: 'inherit',
                fontWeight: 500,
                fontSize: '0.875rem',
                maxHeight: '120px',
                overflowY: 'auto',
              }}
            >
              {status.params}
            </Box>
          </LabeledItem>
        </Box>
      )}
    </Paper>
  );
}

export default DAGStatusOverview;
