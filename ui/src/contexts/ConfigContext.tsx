import { createContext, useContext } from 'react';

export type Config = {
  apiURL: string;
  basePath: string;
  title: string;
  navbarColor: string;
  tz: string;
  version: string;  
  maxDashboardPageLimit: number;
  remoteNodes: string;
};

export const ConfigContext = createContext<Config>(null!);

export function useConfig() {
  return useContext(ConfigContext);
}
