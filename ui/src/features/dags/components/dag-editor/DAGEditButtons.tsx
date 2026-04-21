/**
 * DAGEditButtons component provides buttons for renaming and deleting a DAG.
 *
 * @module features/dags/components/dag-editor
 */
import { useAgentChatContext } from '@/features/agent';
import { useCanWriteForWorkspace } from '@/contexts/AuthContext';
import { Button } from '@/components/ui/button';
import { useErrorModal } from '@/components/ui/error-modal';
import { Sparkles, PencilLine, Trash2 } from 'lucide-react';
import React from 'react';
import { components } from '../../../../api/v1/schema';
import { DAGNameInputModal } from '../../../../components/DAGNameInputModal';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useConfig } from '../../../../contexts/ConfigContext';
import { useUnsavedChanges } from '../../../../contexts/UnsavedChangesContext';
import { useUserPreferences } from '../../../../contexts/UserPreference';
import { useClient } from '../../../../hooks/api';
import ImproveDAGDefinitionModal from './ImproveDAGDefinitionModal';
import { buildImproveDAGDefinitionPrompt } from './improveDagDefinitionPrompt';

/**
 * Props for the DAGEditButtons component
 */
type Props = {
  /** DAG file name */
  fileName: string;
  /** DAG display name */
  dagName?: string;
  /** Latest DAG run details for improvement context */
  latestDAGRun?: components['schemas']['DAGRunDetails'];
  /** Workspace label value for the DAG; empty/null means default. */
  workspace?: string | null;
};

/**
 * DAGEditButtons provides buttons for renaming and deleting a DAG
 */
function DAGEditButtons({ fileName, dagName, latestDAGRun, workspace }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const canWrite = useCanWriteForWorkspace(workspace);
  const client = useClient();
  const config = useConfig();
  const { preferences } = useUserPreferences();
  const { showError } = useErrorModal();
  const { hasUnsavedChanges } = useUnsavedChanges();
  const {
    clearSession: clearAgentSession,
    openChat,
    setPendingUserMessage,
    setSessionId,
    setSessionState,
  } = useAgentChatContext();
  const [isRenameModalOpen, setIsRenameModalOpen] = React.useState(false);
  const [renameError, setRenameError] = React.useState<string | null>(null);
  const [isRenameLoading, setIsRenameLoading] = React.useState(false);
  const [isImproveModalOpen, setIsImproveModalOpen] = React.useState(false);
  const [isLaunchingImproveSession, setIsLaunchingImproveSession] =
    React.useState(false);
  const improveLaunchInFlightRef = React.useRef(false);

  if (!canWrite) {
    return null;
  }

  const handleRenameClose = () => {
    setIsRenameModalOpen(false);
    setRenameError(null);
  };

  const handleRenameSubmit = async (newFileName: string) => {
    setIsRenameLoading(true);
    setRenameError(null);

    try {
      const { error } = await client.POST('/dags/{fileName}/rename', {
        params: {
          path: {
            fileName: fileName,
          },
          query: {
            remoteNode: appBarContext.selectedRemoteNode || 'local',
          },
        },
        body: {
          newFileName: newFileName,
        },
      });

      if (error) {
        setRenameError(error.message || 'An error occurred');
        setIsRenameLoading(false);
        return;
      }

      // Success - close modal and redirect
      setIsRenameModalOpen(false);

      // Redirect to the new DAG page
      const basePath = window.location.pathname.split('/dags')[0] || '';
      window.location.href = `${basePath}/dags/${newFileName}`;
    } catch {
      setRenameError('An unexpected error occurred');
      setIsRenameLoading(false);
    }
  };

  const handleImproveClose = React.useCallback(() => {
    if (isLaunchingImproveSession) {
      return;
    }
    setIsImproveModalOpen(false);
  }, [isLaunchingImproveSession]);

  const handleImproveSubmit = React.useCallback(
    async (userPrompt: string) => {
      if (improveLaunchInFlightRef.current) {
        return;
      }

      if (!config.agentEnabled) {
        showError(
          'Agent is disabled',
          'Enable the agent in settings before starting a DAG improvement session.'
        );
        return;
      }

      if (hasUnsavedChanges) {
        showError(
          'Save or discard local edits first',
          'The agent can only improve the saved DAG definition that exists on disk.'
        );
        return;
      }

      improveLaunchInFlightRef.current = true;
      setIsLaunchingImproveSession(true);

      try {
        clearAgentSession();
        setPendingUserMessage(userPrompt);

        const { data: sessionData, error } = await client.POST('/agent/sessions', {
          params: {
            query: {
              remoteNode: appBarContext.selectedRemoteNode || 'local',
            },
          },
          body: {
            message: buildImproveDAGDefinitionPrompt({
              dagFile: fileName,
              dagName: dagName || fileName,
              latestDAGRun,
              userPrompt,
            }),
            dagContexts: [
              {
                dagFile: fileName,
                dagRunId: latestDAGRun?.dagRunId,
              },
            ],
            safeMode: preferences.safeMode,
          },
        });

        if (error || !sessionData) {
          setPendingUserMessage(null);
          showError(
            error?.message || 'Failed to start improvement session',
            'Please try again after the agent service is available.'
          );
          return;
        }

        setSessionId(sessionData.sessionId);
        setSessionState({
          session_id: sessionData.sessionId,
          working: true,
        });
        setIsImproveModalOpen(false);
        openChat();
      } catch {
        setPendingUserMessage(null);
        showError(
          'Failed to start improvement session',
          'Please try again after the agent service is available.'
        );
      } finally {
        improveLaunchInFlightRef.current = false;
        setIsLaunchingImproveSession(false);
      }
    },
    [
      appBarContext.selectedRemoteNode,
      clearAgentSession,
      client,
      config.agentEnabled,
      dagName,
      fileName,
      hasUnsavedChanges,
      latestDAGRun,
      openChat,
      preferences.safeMode,
      setPendingUserMessage,
      setSessionId,
      setSessionState,
      showError,
    ]
  );

  return (
    <div className="flex items-center gap-2">
      <Button onClick={() => setIsRenameModalOpen(true)}>
        <PencilLine className="h-4 w-4" />
        Rename
      </Button>

      {config.agentEnabled && (
        <Button
          variant="outline"
          title={
            hasUnsavedChanges
              ? 'Save or discard local changes before using agent improvement'
              : 'Start a fresh agent session to improve this DAG definition'
          }
          disabled={hasUnsavedChanges || isLaunchingImproveSession}
          onClick={() => setIsImproveModalOpen(true)}
        >
          <Sparkles className="h-4 w-4" />
          Improve
        </Button>
      )}

      <Button
        variant="destructive"
        onClick={async () => {
          if (!confirm('Are you sure to delete the DAG?')) {
            return;
          }
          const { error } = await client.DELETE('/dags/{fileName}', {
            params: {
              path: {
                fileName: fileName,
              },
              query: {
                remoteNode: appBarContext.selectedRemoteNode || 'local',
              },
            },
          });
          if (error) {
            showError(
              error.message || 'Failed to delete DAG',
              'Please try again or check the server connection.'
            );
            return;
          }
          // Redirect to the DAGs list page
          const basePath = window.location.pathname.split('/dags')[0] || '';
          window.location.href = `${basePath}/dags/`;
        }}
      >
        <Trash2 className="h-4 w-4" />
        Delete
      </Button>

      <DAGNameInputModal
        isOpen={isRenameModalOpen}
        onClose={handleRenameClose}
        onSubmit={handleRenameSubmit}
        mode="rename"
        initialValue={fileName}
        isLoading={isRenameLoading}
        externalError={renameError}
      />
      <ImproveDAGDefinitionModal
        isOpen={isImproveModalOpen}
        onClose={handleImproveClose}
        onSubmit={handleImproveSubmit}
        dagFile={fileName}
        dagName={dagName}
        latestDAGRun={latestDAGRun}
        isLoading={isLaunchingImproveSession}
      />
    </div>
  );
}

export default DAGEditButtons;
