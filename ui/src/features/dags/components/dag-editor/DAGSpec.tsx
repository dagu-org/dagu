// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

/**
 * DAGSpec component displays and allows editing of a DAG specification.
 *
 * @module features/dags/components/dag-editor
 */
import { useCanWrite } from '@/contexts/AuthContext';
import { StepDetails } from '@/features/dags/components/step-details';
import { cn, toMermaidNodeId } from '@/lib/utils';
import BorderedBox from '@/ui/BorderedBox';
import { AlertTriangle, MousePointerClick, Save, Undo2, X } from 'lucide-react';
import React, { useEffect } from 'react';
import { useCookies } from 'react-cookie';
import { createPortal } from 'react-dom';
import { components } from '../../../../api/v1/schema';
import { Button } from '../../../../components/ui/button';
import { useErrorModal } from '../../../../components/ui/error-modal';
import { useSimpleToast } from '../../../../components/ui/simple-toast';
import { Tab, Tabs } from '../../../../components/ui/tabs';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useSchema } from '../../../../contexts/SchemaContext';
import { useUnsavedChanges } from '../../../../contexts/UnsavedChangesContext';
import { useClient, useQuery } from '../../../../hooks/api';
import { useContentEditor } from '../../../../hooks/useContentEditor';
import { useDAGSSE } from '../../../../hooks/useDAGSSE';
import {
  sseFallbackOptions,
  useSSECacheSync,
} from '../../../../hooks/useSSECacheSync';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import { DAGContext } from '../../contexts/DAGContext';
import { DAGStepTable } from '../dag-details';
import { FlowchartType, Graph } from '../visualization';
import {
  buildAugmentedDAGSchema,
  customStepTypeHintsEqual,
  extractLocalCustomStepTypeHints,
  mergeCustomStepTypeHints,
  toInheritedCustomStepTypeHints,
} from './customStepSchema';
import DAGAttributes from './DAGAttributes';
import DAGEditorWithDocs from './DAGEditorWithDocs';
import ExternalChangeDialog from './ExternalChangeDialog';

/**
 * Props for the DAGSpec component
 */
type Props = {
  /** DAG file name */
  fileName: string;
  /** Local DAGs from parent (optional, avoids redundant fetch) */
  localDags?: components['schemas']['LocalDag'][];
  /** Editor-only metadata used for dynamic schema synthesis */
  editorHints?: components['schemas']['DAGEditorHints'];
};

/**
 * DAGSpec displays and allows editing of a DAG specification
 * including visualization, attributes, steps, and YAML definition
 */
function DAGSpec({ fileName, localDags, editorHints }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const editable = useCanWrite();
  const client = useClient();
  const { schema: baseSchema } = useSchema();
  const { showError } = useErrorModal();
  const { showToast } = useSimpleToast();
  const { setHasUnsavedChanges } = useUnsavedChanges();

  const [scrollPosition, setScrollPosition] = React.useState(0);
  const [activeTab, setActiveTab] = React.useState('parent');
  const [selectedSpecStepName, setSelectedSpecStepName] = React.useState<
    string | null
  >(null);
  const [isSpecStepDetailsOpen, setIsSpecStepDetailsOpen] =
    React.useState(false);

  const closeSpecStepDetails = React.useCallback(() => {
    setIsSpecStepDetailsOpen(false);
  }, []);

  const handleActiveTabChange = React.useCallback(
    (tab: string) => {
      setActiveTab(tab);
      setSelectedSpecStepName(null);
      closeSpecStepDetails();
    },
    [closeSpecStepDetails]
  );

  // Flowchart direction preference stored in cookies
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);

  // Reference to the main container div
  const containerRef = React.useRef<HTMLDivElement>(null);

  // Reference to save function and refresh callback for keyboard shortcut
  const saveHandlerRef = React.useRef<(() => Promise<void>) | null>(null);
  const refreshCallbackRef = React.useRef<(() => void) | null>(null);

  /**
   * Handle flowchart direction change and save preference to cookie
   */
  const onChangeFlowchart = React.useCallback(
    (value: FlowchartType) => {
      if (!value) {
        return;
      }
      setCookie('flowchart', value, { path: '/' });
      setFlowchart(value);
    },
    [setCookie, setFlowchart]
  );

  const dagSSE = useDAGSSE(fileName, !!fileName);

  // Fetch spec — SWR is the single source of truth, refreshed by live invalidations
  const {
    data,
    isLoading,
    mutate: mutateSpec,
  } = useQuery(
    '/dags/{fileName}/spec',
    {
      params: {
        query: {
          remoteNode,
        },
        path: {
          fileName: fileName,
        },
      },
    },
    sseFallbackOptions(dagSSE)
  );
  useSSECacheSync(dagSSE, mutateSpec, (next) =>
    next.spec === undefined
      ? undefined
      : {
          dag: next.dag,
          errors: next.errors ?? [],
          spec: next.spec,
        }
  );

  // Server spec — SWR cache stays current via live invalidations or polling fallback
  const serverSpec = data?.spec ?? null;

  // Change tracking (source-agnostic)
  const {
    currentValue,
    setCurrentValue,
    hasUnsavedChanges: localHasUnsavedChanges,
    conflict,
    resolveConflict,
    markAsSaved,
    discardChanges,
  } = useContentEditor({
    key: `${fileName}:${remoteNode}`,
    serverContent: serverSpec,
  });

  const [lastGoodLocalStepTypes, setLastGoodLocalStepTypes] = React.useState(
    () => extractLocalCustomStepTypeHints(serverSpec ?? '').stepTypes
  );

  const inheritedCustomStepTypes = React.useMemo(
    () => toInheritedCustomStepTypeHints(editorHints),
    [editorHints]
  );

  const parsedLocalStepTypes = React.useMemo(
    () => extractLocalCustomStepTypeHints(currentValue ?? serverSpec ?? ''),
    [currentValue, serverSpec]
  );

  useEffect(() => {
    if (!parsedLocalStepTypes.ok) {
      return;
    }
    setLastGoodLocalStepTypes((previous) =>
      customStepTypeHintsEqual(previous, parsedLocalStepTypes.stepTypes)
        ? previous
        : parsedLocalStepTypes.stepTypes
    );
  }, [parsedLocalStepTypes]);

  const effectiveLocalStepTypes = React.useMemo(() => {
    if (!parsedLocalStepTypes.ok) {
      return lastGoodLocalStepTypes;
    }
    return customStepTypeHintsEqual(
      lastGoodLocalStepTypes,
      parsedLocalStepTypes.stepTypes
    )
      ? lastGoodLocalStepTypes
      : parsedLocalStepTypes.stepTypes;
  }, [lastGoodLocalStepTypes, parsedLocalStepTypes]);

  const editorSchema = React.useMemo(() => {
    if (!baseSchema) {
      return null;
    }
    return buildAugmentedDAGSchema(
      baseSchema,
      mergeCustomStepTypeHints(
        inheritedCustomStepTypes,
        effectiveLocalStepTypes
      )
    );
  }, [baseSchema, effectiveLocalStepTypes, inheritedCustomStepTypes]);

  const editorModelUri = React.useMemo(
    () =>
      `inmemory://dagu/${encodeURIComponent(remoteNode)}/dags/${encodeURIComponent(fileName)}.yaml`,
    [fileName, remoteNode]
  );

  // Sync unsaved changes context
  useEffect(() => {
    setHasUnsavedChanges(localHasUnsavedChanges);
  }, [localHasUnsavedChanges, setHasUnsavedChanges]);

  // Clean up unsaved changes state on unmount
  useEffect(() => {
    return () => {
      setHasUnsavedChanges(false);
    };
  }, [setHasUnsavedChanges]);

  // Save scroll position before saving
  const saveScrollPosition = React.useCallback(() => {
    if (containerRef.current) {
      setScrollPosition(window.scrollY);
    }
  }, []);

  // Save handler function
  const handleSave = React.useCallback(async () => {
    if (currentValue == null) {
      showError('No changes to save', 'Make some edits before saving.');
      return;
    }

    // Save current scroll position before any operations that might cause re-render
    saveScrollPosition();

    const { data: responseData, error } = await client.PUT(
      '/dags/{fileName}/spec',
      {
        params: {
          path: {
            fileName: fileName,
          },
          query: {
            remoteNode,
          },
        },
        body: {
          spec: currentValue,
        },
      }
    );

    if (error) {
      showError(
        error.message || 'Failed to save spec',
        'Please check the YAML syntax and try again.'
      );
      return;
    }

    if (responseData?.errors) {
      showError('Validation errors', responseData.errors.join('\n'));
      return;
    }

    // Mark as saved to prevent false conflict detection on our own save
    markAsSaved(currentValue);

    // Revalidate SWR cache from server as safety net
    mutateSpec();

    // Show success toast notification
    showToast('Changes saved successfully');
  }, [
    currentValue,
    fileName,
    remoteNode,
    client,
    saveScrollPosition,
    showError,
    showToast,
    markAsSaved,
    mutateSpec,
  ]);

  // Restore scroll position after render
  useEffect(() => {
    if (scrollPosition > 0) {
      // Use a small timeout to ensure the DOM has updated before scrolling
      const timer = setTimeout(() => {
        window.scrollTo({
          top: scrollPosition,
          behavior: 'auto', // Use 'auto' instead of 'smooth' to avoid animation
        });
      }, 100);

      return () => clearTimeout(timer);
    }
  }, [scrollPosition]);

  // Update save handler ref when handleSave changes
  useEffect(() => {
    saveHandlerRef.current = handleSave;
  }, [handleSave]);

  // Add keyboard shortcut for saving (Ctrl+S / Cmd+S)
  useEffect(() => {
    if (!editable) {
      return;
    }

    const handleKeyDown = async (event: KeyboardEvent) => {
      // Check for Ctrl+S (Windows/Linux) or Cmd+S (macOS)
      if ((event.ctrlKey || event.metaKey) && event.key === 's') {
        event.preventDefault(); // Prevent browser's default save dialog

        // Call the save handler if available
        if (saveHandlerRef.current) {
          await saveHandlerRef.current();

          // Refresh after saving
          if (refreshCallbackRef.current) {
            refreshCallbackRef.current();
          }
        }
      }
    };

    // Add event listener to document
    document.addEventListener('keydown', handleKeyDown);

    // Cleanup on unmount
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [editable]);

  // Show loading indicator while fetching data
  if (isLoading) {
    return <LoadingIndicator />;
  }

  // Check if we have local DAGs
  const hasLocalDags = localDags && localDags.length > 0;

  // Helper function to render DAG content (Graph, Attributes, Steps, Errors)
  const renderDAGContent = (
    dag: components['schemas']['DAGDetails'],
    errors?: string[]
  ) => {
    const selectedStep = selectedSpecStepName
      ? dag.steps?.find((step) => step.name === selectedSpecStepName)
      : undefined;

    const handleGraphNodeSelect = (nodeId: string) => {
      const step = dag.steps?.find(
        (item) => toMermaidNodeId(item.name) === nodeId
      );
      if (!step) {
        return;
      }
      setSelectedSpecStepName(step.name);
      setIsSpecStepDetailsOpen(true);
    };

    return (
      <div className="space-y-6">
        {errors?.length ? (
          <div className="space-y-3">
            {errors.map((e, i) => (
              <div
                key={i}
                className="p-3 bg-danger-highlight rounded-md text-danger font-mono text-sm break-words flex items-start gap-2"
              >
                <AlertTriangle className="h-4 w-4 mt-0.5 flex-shrink-0" />
                {e}
              </div>
            ))}
          </div>
        ) : null}

        {errors?.length || !dag.steps || dag.steps.length === 0 ? (
          <div className="py-8 px-4 text-center">
            <AlertTriangle className="h-12 w-12 text-warning mx-auto mb-4" />
            <p className="text-muted-foreground mb-2">
              Cannot render graph due to configuration errors
            </p>
            <p className="text-sm text-muted-foreground">
              Please fix the errors above and save the configuration to view the
              graph
            </p>
          </div>
        ) : (
          <div>
            <BorderedBox className="py-4 px-4 flex flex-col overflow-x-auto">
              <Graph
                steps={dag.steps}
                type="config"
                flowchart={flowchart}
                onChangeFlowchart={onChangeFlowchart}
                onClickNode={handleGraphNodeSelect}
                selectOnClick
                showIcons={false}
              />
            </BorderedBox>
            <div className="mt-2 flex items-center gap-2 text-xs text-muted-foreground">
              <MousePointerClick className="h-3.5 w-3.5" />
              Select a graph node to inspect its step details.
            </div>
          </div>
        )}

        <DAGAttributes dag={dag} />

        {dag.steps ? (
          <div className="overflow-hidden">
            <DAGStepTable steps={dag.steps} />
          </div>
        ) : null}

        {getHandlers(dag)?.length ? (
          <div className="overflow-hidden">
            <DAGStepTable steps={getHandlers(dag)} />
          </div>
        ) : null}

        <SpecStepDetailsDrawer
          dagName={dag.name}
          isOpen={isSpecStepDetailsOpen}
          step={selectedStep}
          onClose={closeSpecStepDetails}
        />
      </div>
    );
  };

  return (
    <DAGContext.Consumer>
      {(props) => {
        // Update refresh callback ref directly (safe in render)
        refreshCallbackRef.current = props.refresh;

        return (
          data?.dag && (
            <React.Fragment>
              {/* External changes conflict dialog */}
              <ExternalChangeDialog
                visible={conflict.hasConflict}
                onDiscard={() => resolveConflict('discard')}
                onIgnore={() => resolveConflict('ignore')}
              />

              <div
                className="flex flex-col flex-1 min-h-0 space-y-6 mb-6"
                ref={containerRef}
              >
                {hasLocalDags && (
                  <div className="flex-shrink-0">
                    <div className="overflow-x-auto -mx-2 px-2 scrollbar-thin scrollbar-thumb-gray-300">
                      <Tabs className="w-max min-w-full">
                        <Tab
                          isActive={activeTab === 'parent'}
                          onClick={() => handleActiveTabChange('parent')}
                          className="cursor-pointer whitespace-nowrap"
                        >
                          {data?.dag?.name} (Parent)
                        </Tab>
                        {localDags?.map(
                          (localDag: components['schemas']['LocalDag']) => (
                            <Tab
                              key={localDag.name}
                              isActive={activeTab === localDag.name}
                              onClick={() =>
                                handleActiveTabChange(localDag.name)
                              }
                              className="cursor-pointer whitespace-nowrap"
                            >
                              {localDag.name}
                            </Tab>
                          )
                        )}
                      </Tabs>
                    </div>
                  </div>
                )}

                {(() => {
                  if (activeTab === 'parent') {
                    return (
                      data?.dag && (
                        <div className="flex-shrink-0">
                          {renderDAGContent(data.dag, data?.errors)}
                        </div>
                      )
                    );
                  }
                  const selectedLocalDag = localDags?.find(
                    (ld: components['schemas']['LocalDag']) =>
                      ld.name === activeTab
                  );
                  return (
                    selectedLocalDag?.dag && (
                      <div className="flex-shrink-0">
                        {renderDAGContent(
                          selectedLocalDag.dag,
                          selectedLocalDag.errors
                        )}
                      </div>
                    )
                  );
                })()}

                <DAGEditorWithDocs
                  value={
                    editable
                      ? (currentValue ?? serverSpec ?? '')
                      : (serverSpec ?? '')
                  }
                  readOnly={!editable}
                  onChange={
                    editable
                      ? (newValue) => {
                          setCurrentValue(newValue ?? '');
                        }
                      : undefined
                  }
                  className="min-h-[400px]"
                  modelUri={editorModelUri}
                  schema={editorSchema}
                  headerActions={
                    editable ? (
                      <>
                        {localHasUnsavedChanges && (
                          <Button
                            variant="ghost"
                            title="Discard changes"
                            onClick={discardChanges}
                          >
                            <Undo2 className="h-4 w-4" />
                            Discard
                          </Button>
                        )}
                        <Button
                          id="save-config"
                          title="Save changes (Ctrl+S / Cmd+S)"
                          disabled={!localHasUnsavedChanges}
                          onClick={async () => {
                            await handleSave();
                            props.refresh();
                          }}
                        >
                          <Save className="h-4 w-4" />
                          Save
                        </Button>
                      </>
                    ) : undefined
                  }
                />
              </div>
            </React.Fragment>
          )
        );
      }}
    </DAGContext.Consumer>
  );
}

/**
 * Extract lifecycle handlers from DAG definition
 */
function getHandlers(
  dag?: components['schemas']['DAGDetails']
): components['schemas']['Step'][] {
  const steps: components['schemas']['Step'][] = [];
  if (!dag) {
    return steps;
  }
  const h = dag.handlerOn;
  if (h?.success) {
    steps.push(h.success);
  }
  if (h?.failure) {
    steps.push(h?.failure);
  }
  if (h?.abort) {
    steps.push(h?.abort);
  }
  if (h?.exit) {
    steps.push(h?.exit);
  }
  return steps;
}

function SpecStepDetailsDrawer({
  dagName,
  isOpen,
  onClose,
  step,
}: {
  dagName?: string;
  isOpen: boolean;
  onClose: () => void;
  step?: components['schemas']['Step'];
}) {
  const [shouldRender, setShouldRender] = React.useState(false);
  const [isVisible, setIsVisible] = React.useState(false);
  const [renderedStep, setRenderedStep] = React.useState(step);
  const drawerRef = React.useRef<HTMLElement>(null);
  const closeButtonRef = React.useRef<HTMLButtonElement>(null);
  const previouslyFocusedRef = React.useRef<HTMLElement | null>(null);

  React.useEffect(() => {
    let closeTimer: number | undefined;
    let animationFrame: number | undefined;

    if (isOpen && step) {
      previouslyFocusedRef.current =
        document.activeElement instanceof HTMLElement
          ? document.activeElement
          : null;
      setRenderedStep(step);
      setShouldRender(true);
      animationFrame = window.requestAnimationFrame(() => {
        setIsVisible(true);
        closeButtonRef.current?.focus();
      });
    } else {
      setIsVisible(false);
      closeTimer = window.setTimeout(() => {
        setShouldRender(false);
        previouslyFocusedRef.current?.focus();
        previouslyFocusedRef.current = null;
      }, 180);
    }

    return () => {
      if (animationFrame !== undefined) {
        window.cancelAnimationFrame(animationFrame);
      }
      if (closeTimer !== undefined) {
        window.clearTimeout(closeTimer);
      }
    };
  }, [isOpen, step]);

  React.useEffect(() => {
    if (!isOpen) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
        return;
      }

      if (event.key !== 'Tab') {
        return;
      }

      const focusableElements = drawerRef.current?.querySelectorAll<HTMLElement>(
        [
          'a[href]',
          'button:not([disabled])',
          'textarea:not([disabled])',
          'input:not([disabled])',
          'select:not([disabled])',
          '[tabindex]:not([tabindex="-1"])',
        ].join(',')
      );
      if (!focusableElements || focusableElements.length === 0) {
        event.preventDefault();
        return;
      }

      const firstElement = focusableElements.item(0);
      const lastElement = focusableElements.item(focusableElements.length - 1);
      if (!firstElement || !lastElement) {
        event.preventDefault();
        return;
      }

      if (event.shiftKey && document.activeElement === firstElement) {
        event.preventDefault();
        lastElement.focus();
      } else if (!event.shiftKey && document.activeElement === lastElement) {
        event.preventDefault();
        firstElement.focus();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  React.useEffect(() => {
    if (!shouldRender) {
      return;
    }

    const appRoot = document.getElementById('root');
    const previousAriaHidden = appRoot?.getAttribute('aria-hidden') ?? null;
    appRoot?.setAttribute('aria-hidden', 'true');

    return () => {
      if (!appRoot) {
        return;
      }
      if (previousAriaHidden === null) {
        appRoot.removeAttribute('aria-hidden');
      } else {
        appRoot.setAttribute('aria-hidden', previousAriaHidden);
      }
    };
  }, [shouldRender]);

  if (!shouldRender || !renderedStep) {
    return null;
  }

  return createPortal(
    <div className="fixed inset-0 z-[60] flex justify-end">
      <button
        type="button"
        tabIndex={-1}
        aria-label="Close step details"
        className={cn(
          'absolute inset-0 h-full w-full cursor-default bg-black/10 transition-opacity duration-200 ease-out dark:bg-black/20',
          isVisible ? 'opacity-100' : 'opacity-0'
        )}
        onClick={onClose}
      />
      <aside
        ref={drawerRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="spec-step-details-title"
        className={cn(
          'relative z-10 flex h-full w-full max-w-[520px] flex-col border-l border-border bg-background shadow-xl transition-all duration-200 ease-out will-change-transform',
          isVisible ? 'translate-x-0 opacity-100' : 'translate-x-full opacity-0'
        )}
      >
        <header className="flex items-start justify-between gap-4 border-b border-border px-4 py-3">
          <div className="min-w-0">
            <div className="text-xs font-medium uppercase text-muted-foreground">
              {dagName || 'DAG'}
            </div>
            <h2
              id="spec-step-details-title"
              className="mt-1 truncate text-base font-semibold text-foreground"
            >
              {renderedStep.name}
            </h2>
            <p className="mt-1 text-xs text-muted-foreground">
              Selected graph node
            </p>
          </div>
          <Button
            ref={closeButtonRef}
            type="button"
            variant="ghost"
            size="icon"
            onClick={onClose}
            title="Close step details"
          >
            <X className="h-4 w-4" />
          </Button>
        </header>
        <div className="min-h-0 flex-1 overflow-auto p-5">
          <StepDetails step={renderedStep} />
        </div>
      </aside>
    </div>,
    document.body
  );
}

export default DAGSpec;
