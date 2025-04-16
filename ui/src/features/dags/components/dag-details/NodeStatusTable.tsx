/**
 * NodeStatusTable component displays a table of node execution statuses.
 *
 * @module features/dags/components/dag-details
 */
import React, { CSSProperties } from 'react';
import { stepTabColStyles } from '../../../../consts';
import NodeStatusTableRow from './NodeStatusTableRow';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
} from '@mui/material';
import BorderedBox from '../../../../ui/BorderedBox';
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
  const styles = stepTabColStyles;
  let i = 0;

  // Don't render if there are no nodes
  if (!nodes || !nodes.length) {
    return null;
  }

  return (
    <React.Fragment>
      <BorderedBox>
        <Table size="small" sx={tableStyle}>
          <TableHead>
            <TableRow>
              <TableCell style={styles[i++]}>No</TableCell>
              <TableCell style={styles[i++]}>Step Name</TableCell>
              <TableCell style={styles[i++]}>Description</TableCell>
              <TableCell style={styles[i++]}>Command</TableCell>
              <TableCell style={styles[i++]}>Args</TableCell>
              <TableCell style={styles[i++]}>Started At</TableCell>
              <TableCell style={styles[i++]}>Finished At</TableCell>
              <TableCell style={styles[i++]}>Status</TableCell>
              <TableCell style={styles[i++]}>Error</TableCell>
              <TableCell style={styles[i++]}>Log</TableCell>
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
      </BorderedBox>
    </React.Fragment>
  );
}

export default NodeStatusTable;
