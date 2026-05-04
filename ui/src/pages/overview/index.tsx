// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { Tab, Tabs } from '@/components/ui/tabs';
import { Clock3, Gauge } from 'lucide-react';
import React from 'react';
import Dashboard from '..';
import CockpitPage from '../cockpit';

export type OverviewTab = 'timeline' | 'cockpit';

type OverviewPageProps = {
  initialTab?: OverviewTab;
};

const OVERVIEW_ACTIVE_TAB_STORAGE_KEY = 'dagu_overview_active_tab';
const DEFAULT_OVERVIEW_TAB: OverviewTab = 'timeline';

function isOverviewTab(value: string | null | undefined): value is OverviewTab {
  return value === 'timeline' || value === 'cockpit';
}

function readStoredOverviewTab(): OverviewTab | null {
  try {
    const storedTab = localStorage.getItem(OVERVIEW_ACTIVE_TAB_STORAGE_KEY);
    return isOverviewTab(storedTab) ? storedTab : null;
  } catch {
    return null;
  }
}

function getInitialTab(initialTab?: OverviewTab): OverviewTab {
  return initialTab ?? readStoredOverviewTab() ?? DEFAULT_OVERVIEW_TAB;
}

export default function OverviewPage({
  initialTab,
}: OverviewPageProps): React.ReactElement {
  const [activeTab, setActiveTab] = React.useState<OverviewTab>(() =>
    getInitialTab(initialTab)
  );

  React.useEffect(() => {
    if (initialTab) {
      setActiveTab(initialTab);
    }
  }, [initialTab]);

  React.useEffect(() => {
    try {
      localStorage.setItem(OVERVIEW_ACTIVE_TAB_STORAGE_KEY, activeTab);
    } catch {
      /* ignore */
    }
  }, [activeTab]);

  const timelineTabId = 'overview-tab-timeline';
  const cockpitTabId = 'overview-tab-cockpit';
  const timelinePanelId = 'overview-panel-timeline';
  const cockpitPanelId = 'overview-panel-cockpit';

  return (
    <div className="flex h-full min-h-0 flex-col">
      <Tabs
        role="tablist"
        aria-label="Overview views"
        className="mb-3 shrink-0"
      >
        <Tab
          id={timelineTabId}
          role="tab"
          aria-selected={activeTab === 'timeline'}
          aria-controls={timelinePanelId}
          isActive={activeTab === 'timeline'}
          onClick={() => setActiveTab('timeline')}
          className="gap-2 cursor-pointer"
        >
          <Clock3 className="h-4 w-4" />
          Timeline
        </Tab>
        <Tab
          id={cockpitTabId}
          role="tab"
          aria-selected={activeTab === 'cockpit'}
          aria-controls={cockpitPanelId}
          isActive={activeTab === 'cockpit'}
          onClick={() => setActiveTab('cockpit')}
          className="gap-2 cursor-pointer"
        >
          <Gauge className="h-4 w-4" />
          Cockpit
        </Tab>
      </Tabs>

      <div
        id={activeTab === 'timeline' ? timelinePanelId : cockpitPanelId}
        role="tabpanel"
        aria-labelledby={activeTab === 'timeline' ? timelineTabId : cockpitTabId}
        className="min-h-0 flex-1 overflow-hidden"
      >
        {activeTab === 'timeline' ? <Dashboard /> : <CockpitPage />}
      </div>
    </div>
  );
}
