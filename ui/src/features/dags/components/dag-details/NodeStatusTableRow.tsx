/**
 * NodeStatusTableRow component renders a single row in the node status table.
 *
 * @module features/dags/components/dag-details
 */
import React, { useState, useEffect } from 'react';
import MultilineText from '../../../../ui/MultilineText';
import { TableCell, Tooltip, Box, Stack, Typography } from '@mui/material';
import StyledTableRow from '../../../../ui/StyledTableRow';
import {
  ArticleOutlined,
  ErrorOutline,
  Code,
  DescriptionOutlined,
} from '@mui/icons-material';
import { Link } from 'react-router-dom';
import { components } from '../../../../api/v2/schema';
import { NodeStatusChip } from '../common';
import { NodeStatus } from '../../../../api/v2/schema';

/**
 * Props for the NodeStatusTableRow component
 */
type Props = {
  /** Row number for display */
  rownum: number;
  /** Node data to display */
  node: components['schemas']['Node'];
  /** Request ID for log linking */
  requestId?: string;
  /** DAG name/fileId */
  name: string;
};

/**
 * Format timestamp for better readability
 */
const formatTimestamp = (timestamp: string | undefined) => {
  if (!timestamp || timestamp == '-') return '-';
  try {
    const date = new Date(timestamp);
    return date.toLocaleString();
  } catch (e) {
    return timestamp;
  }
};

/**
 * Calculate duration between two timestamps
 * If endTime is not provided, calculate duration from startTime to now (for running tasks)
 */
const calculateDuration = (
  startTime: string | undefined,
  endTime: string | undefined
) => {
  if (!startTime) return '-';

  try {
    const start = new Date(startTime).getTime();
    const end = endTime ? new Date(endTime).getTime() : new Date().getTime();

    if (isNaN(start) || isNaN(end)) return '-';

    const durationMs = end - start;

    // Format duration
    if (durationMs < 0) return '-';
    if (durationMs < 1000) return `${durationMs}ms`;

    const seconds = Math.floor(durationMs / 1000);
    if (seconds < 60) return `${seconds}s`;

    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;
    if (minutes < 60) return `${minutes}m ${remainingSeconds}s`;

    const hours = Math.floor(minutes / 60);
    const remainingMinutes = minutes % 60;
    return `${hours}h ${remainingMinutes}m ${remainingSeconds}s`;
  } catch (e) {
    return '-';
  }
};

/**
 * NodeStatusTableRow displays information about a single node's execution status
 */
function NodeStatusTableRow({ name, rownum, node, requestId }: Props) {
  // State to store the current duration for running tasks
  const [currentDuration, setCurrentDuration] = useState<string>('-');

  // Update duration every second for running tasks
  useEffect(() => {
    if (node.status === NodeStatus.Running && node.startedAt) {
      // Initial calculation
      setCurrentDuration(calculateDuration(node.startedAt, node.finishedAt));

      // Set up interval to update duration every second
      const intervalId = setInterval(() => {
        setCurrentDuration(calculateDuration(node.startedAt, node.finishedAt));
      }, 1000);

      // Clean up interval on unmount or when status changes
      return () => clearInterval(intervalId);
    } else {
      // For non-running tasks, calculate once
      setCurrentDuration(calculateDuration(node.startedAt, node.finishedAt));
    }
  }, [node.status, node.startedAt, node.finishedAt]);
  // Build URL for log viewing
  const searchParams = new URLSearchParams();
  searchParams.set('remoteNode', 'local');
  if (node.step) {
    searchParams.set('step', node.step.name);
  }
  if (requestId) {
    searchParams.set('requestId', requestId);
  }

  const url = `/dags/${name}/log?${searchParams.toString()}`;

  // Extract arguments for display
  let args = '';
  if (node.step.args) {
    // Use uninterpolated args to avoid render issues with very long params
    args =
      node.step.cmdWithArgs?.replace(node.step.command || '', '').trimStart() ||
      '';
  }

  // Determine row highlight based on status
  const getRowHighlight = () => {
    switch (node.status) {
      case NodeStatus.Running:
        return 'rgba(0, 255, 0, 0.05)';
      case NodeStatus.Failed:
        return 'rgba(255, 0, 0, 0.05)';
      default:
        return undefined;
    }
  };

  return (
    <StyledTableRow sx={{ backgroundColor: getRowHighlight() }}>
      <TableCell align="center">
        <Box sx={{ fontWeight: 600 }}>{rownum}</Box>
      </TableCell>

      {/* Combined Step Name & Description */}
      <TableCell>
        <Stack spacing={0.5}>
          <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
            {node.step.name}
          </Typography>
          {node.step.description && (
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ fontSize: '0.85rem' }}
            >
              {node.step.description}
            </Typography>
          )}
        </Stack>
      </TableCell>

      {/* Combined Command & Args */}
      <TableCell>
        <Stack spacing={0.5}>
          {!node.step.command && node.step.cmdWithArgs ? (
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                fontWeight: 500,
                fontSize: '0.85rem',
              }}
            >
              <Code fontSize="small" color="primary" />
              <Box
                component="span"
                sx={{
                  backgroundColor: 'rgba(0, 0, 0, 0.04)',
                  borderRadius: '4px',
                  padding: '2px 4px',
                }}
              >
                {node.step.cmdWithArgs}
              </Box>
            </Box>
          ) : null}

          {node.step.command && (
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                fontWeight: 500,
                fontSize: '0.85rem',
              }}
            >
              <Code fontSize="small" color="primary" />
              <Box
                component="span"
                sx={{
                  backgroundColor: 'rgba(0, 0, 0, 0.04)',
                  borderRadius: '4px',
                  padding: '2px 4px',
                }}
              >
                {node.step.command}
              </Box>
            </Box>
          )}

          {args && (
            <Box
              sx={{
                fontWeight: 500,
                fontSize: '0.85rem',
                pl: 3,
                maxWidth: '100%',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
                color: 'text.secondary',
              }}
            >
              {args}
            </Box>
          )}
        </Stack>
      </TableCell>

      {/* Last Run & Duration */}
      <TableCell>
        <Stack spacing={0.5}>
          <Box sx={{ fontWeight: 500 }}>{formatTimestamp(node.startedAt)}</Box>
          {node.startedAt && (
            <Box
              sx={{
                fontSize: '0.85rem',
                color: 'text.secondary',
                display: 'flex',
                alignItems: 'center',
                gap: 0.5,
              }}
            >
              <Box
                component="span"
                sx={{ fontWeight: 500, display: 'flex', alignItems: 'center' }}
              >
                Duration:
                {node.status === NodeStatus.Running && (
                  <Box
                    component="span"
                    sx={{
                      display: 'inline-block',
                      width: '8px',
                      height: '8px',
                      borderRadius: '50%',
                      backgroundColor: 'lime',
                      ml: 0.5,
                      animation: 'pulse 1.5s infinite',
                    }}
                  />
                )}
              </Box>
              {currentDuration}
            </Box>
          )}
        </Stack>
      </TableCell>

      {/* Status */}
      <TableCell style={{ textAlign: 'center' }}>
        <NodeStatusChip status={node.status} size="sm">
          {node.statusText}
        </NodeStatusChip>
      </TableCell>

      {/* Error */}
      <TableCell>
        {node.error && (
          <Box
            sx={{
              fontSize: '0.85rem',
              backgroundColor: 'rgba(255, 0, 0, 0.05)',
              border: '1px solid rgba(255, 0, 0, 0.1)',
              borderRadius: '4px',
              padding: '4px 8px',
              maxHeight: '80px',
              overflowY: 'auto',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
            }}
          >
            {node.error}
          </Box>
        )}
      </TableCell>

      {/* Log */}
      <TableCell align="center">
        {node.log ? (
          <Link to={url} style={{ textDecoration: 'none' }}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                padding: '6px 12px',
                transition: 'all 0.2s',
                textDecoration: 'none',
                borderRadius: '4px',
                color: 'rgba(0, 0, 0, 0.7)',
                fontWeight: 500,
              }}
            >
              <Tooltip title="View Log">
                <DescriptionOutlined fontSize="small" />
              </Tooltip>
            </Box>
          </Link>
        ) : null}
      </TableCell>
    </StyledTableRow>
  );
}

export default NodeStatusTableRow;
