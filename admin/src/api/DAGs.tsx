import { DAGStatus } from '../models/DAGData';
import { Group } from '../models/Group';

export type GetDAGsResponse = {
  Title: string;
  Charset: string;
  DAGs: DAGStatus[];
  Groups: Group[];
  Group: string;
  Errors: string[];
  HasError: boolean;
};
