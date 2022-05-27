import { Condition } from "./Condition";

export type Step = {
  Name: string;
  Description: string;
  Variables: string[];
  Dir: string;
  CmdWithArgs: string;
  Command: string;
  Script: string;
  Stdout: string;
  Output: string;
  Args: string[];
  Depends: string[];
  ContinueOn: ContinueOn;
  RetryPolicy?: RetryPolicy;
  RepeatPolicy: RepeatPolicy;
  MailOnError: boolean;
  Preconditions: Condition[];
};

export type RetryPolicy = {
  Limit: number;
};

export type RepeatPolicy = {
  Repeat: boolean;
  Interval: number;
};

export type ContinueOn = {
  Failure: boolean;
  Skipped: boolean;
};
