/**
 * NodeStatusTable component displays a table of node execution statuses.
 *
 * @module features/dags/components/dag-details
 */
import React, { CSSProperties } from 'react';
import NodeStatusTableRow from './NodeStatusTableRow';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Paper,
  Box,
} from '@mui/material';
import { components } from '../../../../api/v2/schema';

/**
 * Props for the NodeStatusTable component
 */
type Props = {
  /** List of nodes to display */
  nodes?: components['schemas']['Node'][];
  /** DAG run details */
  status: components['schemas']['RunDetails'];
  /** DAG file ID */
  fileId: string;
};

/**
 * Table style for fixed layout and word wrapping
 */
const tableStyle: CSSProperties = {
  tableLayout: 'fixed',
  wordWrap: 'break-word',
};

/**
 * NodeStatusTable displays execution status information for all nodes in a DAG run
 */
function NodeStatusTable({ nodes, status, fileId }: Props) {
  // Don't render if there are no nodes
  if (!nodes || !nodes.length) {
    return null;
  }

  return (
    <Paper
      elevation={0}
      sx={{
        borderRadius: 2,
        border: '1px solid rgba(0, 0, 0, 0.12)',
        overflow: 'hidden',
        transition: 'all 0.2s ease-in-out',
        '&:hover': {
          boxShadow: '0px 4px 8px rgba(0, 0, 0, 0.05)',
        },
        mb: 3,
      }}
    >
      <Box sx={{ overflowX: 'auto' }}>
        <Table
          size="small"
          sx={{
            ...tableStyle,
            '& .MuiTableCell-head': {
              fontWeight: 600,
            },
          }}
        >
          <TableHead>
            <TableRow>
              <TableCell
                style={{
                  width: '5%',
                  maxWidth: '50px',
                }}
              >
                No
              </TableCell>
              <TableCell
                style={{
                  width: '20%',
                  maxWidth: '200px',
                }}
              >
                Step Name
              </TableCell>
              <TableCell
                style={{
                  width: '20%',
                  maxWidth: '200px',
                }}
              >
                Command
              </TableCell>
              <TableCell
                style={{
                  width: '20%',
                  maxWidth: '200px',
                }}
              >
                Last Run
              </TableCell>
              <TableCell
                style={{
                  width: '20%',
                  maxWidth: '200px',
                  textAlign: 'center',
                }}
              >
                Status
              </TableCell>
              <TableCell
                style={{
                  width: '20%',
                  maxWidth: '200px',
                }}
              >
                Error
              </TableCell>
              <TableCell
                style={{
                  width: '20%',
                  maxWidth: '200px',
                  textAlign: 'center',
                }}
              >
                Log
              </TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {nodes.map((n, idx) => (
              <NodeStatusTableRow
                key={n.step.name}
                rownum={idx + 1}
                node={n}
                requestId={status.requestId}
                name={fileId}
              />
            ))}
          </TableBody>
        </Table>
      </Box>
    </Paper>
  );
}

export default NodeStatusTable;
