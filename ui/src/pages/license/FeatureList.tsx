// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { Check } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useConfig, type LicenseStatus } from '@/contexts/ConfigContext';
import { useI18n } from '@/i18n/I18nProvider';
import { hasLicensedFeature, licensedFeatures } from '@/lib/license';

/**
 * Lists the licensed features, whether this server's license includes each,
 * and where to open or set up the included ones.
 */
export function FeatureList({ license }: { license: LicenseStatus }) {
  const { ts } = useI18n();
  const config = useConfig();
  return (
    <section
      id="features"
      className="card-obsidian scroll-mt-4"
      aria-labelledby="license-features"
    >
      <h2 id="license-features" className="px-4 pt-4 pb-2 text-sm font-medium">
        {ts('Features')}
      </h2>
      <ul className="divide-y divide-border border-t border-border">
        {licensedFeatures.map((feature) => {
          const included = hasLicensedFeature(license, feature.id);
          const setup =
            feature.id === 'sso' ||
            (feature.id === 'rbac' && config.authMode !== 'builtin');
          const href =
            feature.id === 'rbac' && setup
              ? 'https://docs.dagu.sh/server-admin/authentication/builtin'
              : feature.href;
          return (
            <li
              key={feature.id}
              className="flex flex-col gap-1 px-4 py-2 text-sm sm:flex-row sm:items-center sm:gap-x-4"
            >
              <div className="min-w-0 sm:flex-1">
                <h3 className="text-sm font-medium">{ts(feature.title)}</h3>
                <p className="text-xs text-muted-foreground">
                  {ts(feature.description)}
                </p>
              </div>
              <div className="flex items-center gap-4 text-xs sm:contents">
                <span
                  className={
                    included
                      ? 'inline-flex items-center gap-1 text-success sm:w-24 sm:shrink-0'
                      : 'text-muted-foreground sm:w-24 sm:shrink-0'
                  }
                >
                  {included && <Check className="h-3 w-3" aria-hidden="true" />}
                  {ts(included ? 'Included' : 'Not included')}
                </span>
                <span className="sm:w-20 sm:shrink-0 sm:text-right">
                  {included &&
                    (setup ? (
                      <a
                        className="underline"
                        href={href}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        {ts('Setup guide')}
                      </a>
                    ) : (
                      <Link className="underline" to={href}>
                        {ts('Open feature')}
                      </Link>
                    ))}
                </span>
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
