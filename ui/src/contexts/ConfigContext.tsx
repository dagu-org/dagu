import { createContext, useContext } from 'react';

export type PathsConfig = {
  dagsDir: string;
  logDir: string;
  suspendFlagsDir: string;
  adminLogsDir: string;
  baseConfig: string;
  dagRunsDir: string;
  queueDir: string;
  procDir: string;
  serviceRegistryDir: string;
  configFileUsed: string;
  gitSyncDir: string;
  auditLogsDir: string;
};

export type AuthMode = 'none' | 'builtin' | 'oidc' | '';

export type Config = {
  apiURL: string;
  basePath: string;
  title: string;
  navbarColor: string;
  tz: string;
  tzOffsetInSec: number | undefined;
  version: string;
  maxDashboardPageLimit: number;
  remoteNodes: string;
  authMode: AuthMode;
  oidcEnabled: boolean;
  oidcButtonLabel: string;
  terminalEnabled: boolean;
  gitSyncEnabled: boolean;
  permissions: {
    writeDags: boolean;
    runDags: boolean;
  };
  paths: PathsConfig;
};

export const ConfigContext = createContext<Config>(null!);

export function useConfig() {
  return useContext(ConfigContext);
}
