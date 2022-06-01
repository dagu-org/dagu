import { Config } from "./Config";
import { Status } from "./Status";

export type DAG = {
  File: string;
  Dir: string;
  Config: Config;
  Status?: Status;
  Error?: any;
  ErrorT: string;
};
