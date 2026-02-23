import { useConfig } from '@/contexts/ConfigContext';
import type { LicenseStatus } from '@/contexts/ConfigContext';

export function useLicense(): LicenseStatus {
  const config = useConfig();
  return config.license;
}

export function useHasFeature(feature: string): boolean {
  const license = useLicense();
  if (license.community) return true;
  return license.features.includes(feature);
}
