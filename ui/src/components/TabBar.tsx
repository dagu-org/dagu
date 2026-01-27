import * as TabsPrimitive from '@radix-ui/react-tabs';
import { Plus, X } from 'lucide-react';
import React from 'react';
import { useTabContext } from '../contexts/TabContext';
import './TabBar.css';

interface TabBarProps {
  onAddTab?: () => void;
}

function TabBar({ onAddTab }: TabBarProps) {
  const { tabs, activeTabId, setActiveTab, closeTab } = useTabContext();

  if (tabs.length === 0) {
    return null;
  }

  return (
    <TabsPrimitive.Root
      value={activeTabId || ''}
      onValueChange={setActiveTab}
    >
      <div className="tab-bar-wrapper">
        <TabsPrimitive.List className="tab-list">
          {tabs.map((tab) => (
            <TabsPrimitive.Trigger
              key={tab.id}
              value={tab.id}
              className="tab-trigger"
              asChild
            >
              <div>
                {/* Right angled edge for trapezoid shape */}
                <span className="tab-edge-right" />
                <span className="tab-title">{tab.title}</span>
                <button
                  onClick={(e) => {
                    e.stopPropagation();
                    closeTab(tab.id);
                  }}
                  className="tab-close"
                  title="Close"
                >
                  <X className="h-3 w-3" />
                </button>
              </div>
            </TabsPrimitive.Trigger>
          ))}
          {onAddTab && (
            <button
              onClick={onAddTab}
              className="tab-add"
              title="New tab"
            >
              <Plus className="h-4 w-4" />
            </button>
          )}
        </TabsPrimitive.List>
      </div>
    </TabsPrimitive.Root>
  );
}

export default TabBar;
