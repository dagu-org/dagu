import { useConfig } from '@/contexts/ConfigContext';
import type { LicenseStatus } from '@/contexts/ConfigContext';

const defaultLicense: LicenseStatus = {
  valid: false,
  plan: '',
  expiry: '',
  features: [],
  gracePeriod: false,
  community: true,
  source: '',
  warningCode: '',
};

export function useLicense(): LicenseStatus {
  const config = useConfig();
  return config?.license ?? defaultLicense;
}

export function useHasFeature(feature: string): boolean {
  const license = useLicense();
  return license.features.includes(feature);
}
