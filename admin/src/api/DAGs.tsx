import { DAGStatus } from '../models';
import { Group } from '../models';

export type GetDAGsResponse = {
  Title: string;
  Charset: string;
  DAGs: DAGStatus[];
  Groups: Group[];
  Group: string;
  Errors: string[];
  HasError: boolean;
};
