import { DAG } from "../models/DAG";
import { Group } from "../models/Group";

export type GetDAGsResponse = {
  Title: string;
  Charset: string;
  DAGs: DAG[];
  Groups: Group[];
  Group: string;
  Errors: string[];
  HasError: boolean;
};
