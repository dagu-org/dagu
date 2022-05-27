import { Condition } from "./Condition";
import { Step } from "./Step";

export type Config = {
  ConfigPath: string;
  Name: string;
  Description: string;
  Env: string[];
  LogDir: string;
  HandlerOn: HandlerOn;
  Steps: Step[];
  HistRetentionDays: number;
  Preconditions: Condition[];
  MaxActiveRuns: number;
  Params: string[];
  DefaultParams: string;
	Delay             :number
	MaxCleanUpTime    :number
};

export type HandlerOn = {
  Failure: Step;
  Success: Step;
  Cancel: Step;
  Exit: Step;
};
