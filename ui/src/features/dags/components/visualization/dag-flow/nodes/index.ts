import { StepNode } from './StepNode';
import { SubDagNode } from './SubDagNode';
import { ParallelNode } from './ParallelNode';

export const nodeTypes = {
  step: StepNode,
  subdag: SubDagNode,
  parallel: ParallelNode,
};

export { StepNode, SubDagNode, ParallelNode };
