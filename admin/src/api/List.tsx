import { DAG } from "../models/Dag";
import { Group } from "../models/Group";

export type GetListResponse = {
  Title: string;
  Charset: string;
  DAGs: DAG[];
  Groups: Group[];
  Group: string;
  Errors: string[];
  HasError: boolean;
};
