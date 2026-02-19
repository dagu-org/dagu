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

export type AuthMode = 'none' | 'basic' | 'builtin' | '';

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
  setupRequired: boolean;
  oidcEnabled: boolean;
  oidcButtonLabel: string;
  terminalEnabled: boolean;
  gitSyncEnabled: boolean;
  agentEnabled: boolean;
  updateAvailable: boolean;
  latestVersion: string;
  permissions: {
    writeDags: boolean;
    runDags: boolean;
  };
  paths: PathsConfig;
};

export const ConfigContext = createContext<Config>(null!);

/**
 * Access the application configuration from the nearest ConfigContext provider.
 *
 * @returns The Config object containing API URL, paths, permissions, and feature flags.
 */
export function useConfig(): Config {
  return useContext(ConfigContext);
}
