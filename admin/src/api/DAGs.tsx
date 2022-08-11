import { DAGStatus } from '../models';

export type GetDAGsResponse = {
  Title: string;
  Charset: string;
  DAGs: DAGStatus[];
  Errors: string[];
  HasError: boolean;
};
