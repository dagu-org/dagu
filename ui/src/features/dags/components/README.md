# DAG Components

This directory contains components for the DAG (Directed Acyclic Graph) feature of the application. The components have been organized into logical groups to improve maintainability and code organization.

## Directory Structure

```
src/features/dags/components/
├── common/             # Shared components specific to DAGs
├── dag-details/        # Components for viewing DAG details
├── dag-execution/      # Components for execution history and logs
├── dag-list/           # Components for listing and filtering DAGs
├── dag-editor/         # Components for editing DAG definitions
└── visualization/      # Graph and timeline visualization components
```

## Component Groups

### Common Components

Components that are used across multiple features:

- `DAGActions`: Provides action buttons for DAG operations (start, stop, retry)
- `LiveSwitch`: Toggle switch for enabling/disabling DAG scheduling

### DAG List Components

Components for listing and managing DAGs:

- `DAGTable`: Table component for displaying DAGs with filtering, sorting, and grouping

### DAG Execution Components

Components for DAG execution management:

- `StartDAGModal`: Modal dialog for starting a DAG with parameters

### Visualization Components

Components for visualizing DAG workflows:

- `DAGGraph`: Tabbed interface for visualizing DAG runs as either a graph or timeline
- `Graph`: Renders a Mermaid.js flowchart of DAG steps with dependencies
- `TimelineChart`: Renders a Gantt chart showing the execution timeline of DAG steps
- `FlowchartSwitch`: Toggle for switching between horizontal and vertical flowchart layouts

## Usage

Import components using the index files to take advantage of the organized structure:

```typescript
// Import from the main components index
import { DAGTable, DAGActions, DAGGraph } from '../features/dags/components';

// Or import from specific feature groups
import {
  Graph,
  TimelineChart,
} from '../features/dags/components/visualization';
import { DAGActions, LiveSwitch } from '../features/dags/components/common';
```

## Best Practices

1. **Component Placement**: Place new components in the appropriate directory based on their functionality.
2. **Export Components**: Add new components to the relevant index.ts file.
3. **Documentation**: Include JSDoc comments for all components and their props.
4. **Consistent Naming**: Follow the established naming conventions.
5. **Separation of Concerns**: Keep components focused on a single responsibility.
