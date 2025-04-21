import { DAG, Schedule } from './index';

export type GetSearchResponse = {
  Errors: string[];
  Results: SearchResult[];
};

export type SearchResult = {
  Name: string;
  DAG?: DAG;
  Matches: Match[];
};

export type Match = {
  Line: string;
  LineNumber: number;
  StartLine: number;
};

export type Workflow = {
  Name: string;
  Group: string;
  Tags: string[];
  Description: string;
  Params: string[];
  DefaultParams?: string;
  Schedule: Schedule[];
};
