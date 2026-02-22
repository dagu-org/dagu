import { useCallback, useContext, useEffect, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ArrowLeft, Save } from 'lucide-react';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { useSimpleToast } from '@/components/ui/simple-toast';
import { useErrorModal } from '@/components/ui/error-modal';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import MarkdownEditor from '@/components/editors/MarkdownEditor';

function generateSlugId(name: string): string {
  return name
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');
}

export default function SoulEditorPage() {
  const { soulId } = useParams<{ soulId: string }>();
  const navigate = useNavigate();
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const { showToast } = useSimpleToast();
  const { showError } = useErrorModal();

  const isCreating = !soulId;
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const [name, setName] = useState('');
  const [idField, setIdField] = useState('');
  const [customId, setCustomId] = useState(false);
  const [description, setDescription] = useState('');
  const [content, setContent] = useState('');
  const [version, setVersion] = useState('');
  const [author, setAuthor] = useState('');
  const [tagsInput, setTagsInput] = useState('');
  const [isLoading, setIsLoading] = useState(!isCreating);
  const [isSaving, setIsSaving] = useState(false);

  const saveHandlerRef = useRef<(() => Promise<void>) | undefined>(undefined);

  useEffect(() => {
    appBarContext.setTitle(isCreating ? 'Create Soul' : 'Edit Soul');
  }, [appBarContext, isCreating]);

  // Fetch soul data in edit mode
  useEffect(() => {
    if (isCreating || !soulId) return;

    (async () => {
      const { data, error } = await client.GET('/settings/agent/souls/{soulId}', {
        params: { path: { soulId }, query: { remoteNode } },
      });
      if (error) {
        showError(error.message || 'Failed to load soul');
        navigate('/agent-souls');
        return;
      }
      setName(data.name);
      setIdField(data.id);
      setDescription(data.description ?? '');
      setContent(data.content ?? '');
      setVersion(data.version ?? '');
      setAuthor(data.author ?? '');
      setTagsInput(data.tags?.join(', ') ?? '');
      setIsLoading(false);
    })();
  }, [soulId, isCreating, client, remoteNode, showError, navigate]);

  function parseTags(input: string): string[] {
    return input
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);
  }

  const handleSave = useCallback(async () => {
    if (!name || !content) return;
    setIsSaving(true);

    try {
      const tags = parseTags(tagsInput);
      if (isCreating) {
        const { error } = await client.POST('/settings/agent/souls', {
          params: { query: { remoteNode } },
          body: {
            id: idField || undefined,
            name,
            content,
            description: description || undefined,
            version: version || undefined,
            author: author || undefined,
            tags: tags.length > 0 ? tags : undefined,
          },
        });
        if (error) throw new Error(error.message || 'Failed to create soul');
        showToast('Soul created');
      } else {
        const { error } = await client.PATCH('/settings/agent/souls/{soulId}', {
          params: { path: { soulId: soulId! }, query: { remoteNode } },
          body: {
            name,
            content,
            description: description || undefined,
            version: version || undefined,
            author: author || undefined,
            tags: tags.length > 0 ? tags : undefined,
          },
        });
        if (error) throw new Error(error.message || 'Failed to update soul');
        showToast('Soul saved');
      }
      navigate('/agent-souls');
    } catch (err) {
      showError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setIsSaving(false);
    }
  }, [
    name, content, description, version, author, tagsInput, idField,
    isCreating, soulId, client, remoteNode, showToast, showError, navigate,
  ]);

  useEffect(() => {
    saveHandlerRef.current = handleSave;
  }, [handleSave]);

  // Ctrl+S / Cmd+S keyboard shortcut
  useEffect(() => {
    const handleKeyDown = async (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key === 's') {
        event.preventDefault();
        if (saveHandlerRef.current) {
          await saveHandlerRef.current();
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);

  if (isLoading) {
    return null;
  }

  return (
    <div className="flex flex-col h-full overflow-hidden -m-4 md:-m-6">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 border-b">
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => navigate('/agent-souls')}
          >
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-sm font-semibold">
            {isCreating ? 'Create Soul' : `Edit: ${name}`}
          </h1>
        </div>
        <div className="flex items-center gap-3">
          <Button
            size="sm"
            className="h-8"
            onClick={handleSave}
            disabled={isSaving || !name || !content}
          >
            <Save className="h-4 w-4 mr-1.5" />
            {isSaving ? 'Saving...' : 'Save'}
          </Button>
        </div>
      </div>

      {/* Metadata panel */}
      <div className="px-4 py-3 border-b space-y-2">
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1">
            <Label htmlFor="soul-name" className="text-xs">Name</Label>
            <Input
              id="soul-name"
              value={name}
              onChange={(e) => {
                const v = e.target.value;
                setName(v);
                if (isCreating && !customId) {
                  setIdField(generateSlugId(v));
                }
              }}
              placeholder="My Soul"
              className="h-7 text-sm"
              required
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="soul-description" className="text-xs">Description</Label>
            <Input
              id="soul-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What this soul defines"
              className="h-7 text-sm"
            />
          </div>
        </div>
        <div className="grid grid-cols-4 gap-3">
          {isCreating && (
            <div className="space-y-1">
              <Label htmlFor="soul-id" className="text-xs">ID</Label>
              <Input
                id="soul-id"
                value={idField}
                onChange={(e) => {
                  setIdField(e.target.value);
                  setCustomId(true);
                }}
                placeholder="auto-generated"
                className="h-7 text-sm"
              />
            </div>
          )}
          <div className="space-y-1">
            <Label htmlFor="soul-version" className="text-xs">Version</Label>
            <Input
              id="soul-version"
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="1.0.0"
              className="h-7 text-sm"
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="soul-author" className="text-xs">Author</Label>
            <Input
              id="soul-author"
              value={author}
              onChange={(e) => setAuthor(e.target.value)}
              placeholder="Author name"
              className="h-7 text-sm"
            />
          </div>
          <div className={`space-y-1 ${isCreating ? '' : 'col-span-2'}`}>
            <Label htmlFor="soul-tags" className="text-xs">Tags</Label>
            <Input
              id="soul-tags"
              value={tagsInput}
              onChange={(e) => setTagsInput(e.target.value)}
              placeholder="personality, assistant, devops"
              className="h-7 text-sm"
            />
          </div>
        </div>
      </div>

      {/* Monaco editor */}
      <div className="flex-1 min-h-0">
        <MarkdownEditor
          value={content}
          onChange={(v) => setContent(v ?? '')}
        />
      </div>
    </div>
  );
}
