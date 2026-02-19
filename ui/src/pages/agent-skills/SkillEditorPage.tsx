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

export default function SkillEditorPage() {
  const { skillId } = useParams<{ skillId: string }>();
  const navigate = useNavigate();
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const { showToast } = useSimpleToast();
  const { showError } = useErrorModal();

  const isCreating = !skillId;
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const [name, setName] = useState('');
  const [idField, setIdField] = useState('');
  const [customId, setCustomId] = useState(false);
  const [description, setDescription] = useState('');
  const [knowledge, setKnowledge] = useState('');
  const [version, setVersion] = useState('');
  const [author, setAuthor] = useState('');
  const [tagsInput, setTagsInput] = useState('');
  const [isLoading, setIsLoading] = useState(!isCreating);
  const [isSaving, setIsSaving] = useState(false);

  const saveHandlerRef = useRef<(() => Promise<void>) | undefined>(undefined);

  useEffect(() => {
    appBarContext.setTitle(isCreating ? 'Create Skill' : 'Edit Skill');
  }, [appBarContext, isCreating]);

  // Fetch skill data in edit mode
  useEffect(() => {
    if (isCreating || !skillId) return;

    (async () => {
      const { data, error } = await client.GET('/settings/agent/skills/{skillId}', {
        params: { path: { skillId }, query: { remoteNode } },
      });
      if (error) {
        showError(error.message || 'Failed to load skill');
        navigate('/agent-skills');
        return;
      }
      setName(data.name);
      setIdField(data.id);
      setDescription(data.description ?? '');
      setKnowledge(data.knowledge ?? '');
      setVersion(data.version ?? '');
      setAuthor(data.author ?? '');
      setTagsInput(data.tags?.join(', ') ?? '');
      setIsLoading(false);
    })();
  }, [skillId, isCreating, client, remoteNode, showError, navigate]);

  function parseTags(input: string): string[] {
    return input
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);
  }

  const handleSave = useCallback(async () => {
    if (!name || !knowledge) return;
    setIsSaving(true);

    try {
      const tags = parseTags(tagsInput);
      if (isCreating) {
        const { error } = await client.POST('/settings/agent/skills', {
          params: { query: { remoteNode } },
          body: {
            id: idField || undefined,
            name,
            knowledge,
            description: description || undefined,
            version: version || undefined,
            author: author || undefined,
            tags: tags.length > 0 ? tags : undefined,
          },
        });
        if (error) throw new Error(error.message || 'Failed to create skill');
        showToast('Skill created');
      } else {
        const { error } = await client.PATCH('/settings/agent/skills/{skillId}', {
          params: { path: { skillId: skillId! }, query: { remoteNode } },
          body: {
            name,
            knowledge,
            description: description || undefined,
            version: version || undefined,
            author: author || undefined,
            tags: tags.length > 0 ? tags : undefined,
          },
        });
        if (error) throw new Error(error.message || 'Failed to update skill');
        showToast('Skill saved');
      }
      navigate('/agent-skills');
    } catch (err) {
      showError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setIsSaving(false);
    }
  }, [
    name, knowledge, description, version, author, tagsInput, idField,
    isCreating, skillId, client, remoteNode, showToast, showError, navigate,
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
            onClick={() => navigate('/agent-skills')}
          >
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-sm font-semibold">
            {isCreating ? 'Create Skill' : `Edit: ${name}`}
          </h1>
        </div>
        <Button
          size="sm"
          className="h-8"
          onClick={handleSave}
          disabled={isSaving || !name || !knowledge}
        >
          <Save className="h-4 w-4 mr-1.5" />
          {isSaving ? 'Saving...' : 'Save'}
        </Button>
      </div>

      {/* Metadata panel */}
      <div className="px-4 py-3 border-b space-y-2">
        <div className="grid grid-cols-2 gap-3">
          <div className="space-y-1">
            <Label htmlFor="skill-name" className="text-xs">Name</Label>
            <Input
              id="skill-name"
              value={name}
              onChange={(e) => {
                const v = e.target.value;
                setName(v);
                if (isCreating && !customId) {
                  setIdField(generateSlugId(v));
                }
              }}
              placeholder="My Skill"
              className="h-7 text-sm"
              required
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="skill-description" className="text-xs">Description</Label>
            <Input
              id="skill-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What this skill does"
              className="h-7 text-sm"
            />
          </div>
        </div>
        <div className="grid grid-cols-4 gap-3">
          {isCreating && (
            <div className="space-y-1">
              <Label htmlFor="skill-id" className="text-xs">ID</Label>
              <Input
                id="skill-id"
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
            <Label htmlFor="skill-version" className="text-xs">Version</Label>
            <Input
              id="skill-version"
              value={version}
              onChange={(e) => setVersion(e.target.value)}
              placeholder="1.0.0"
              className="h-7 text-sm"
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="skill-author" className="text-xs">Author</Label>
            <Input
              id="skill-author"
              value={author}
              onChange={(e) => setAuthor(e.target.value)}
              placeholder="Author name"
              className="h-7 text-sm"
            />
          </div>
          <div className={`space-y-1 ${isCreating ? '' : 'col-span-2'}`}>
            <Label htmlFor="skill-tags" className="text-xs">Tags</Label>
            <Input
              id="skill-tags"
              value={tagsInput}
              onChange={(e) => setTagsInput(e.target.value)}
              placeholder="kubernetes, devops, monitoring"
              className="h-7 text-sm"
            />
          </div>
        </div>
      </div>

      {/* Monaco editor */}
      <div className="flex-1 min-h-0">
        <MarkdownEditor
          value={knowledge}
          onChange={(v) => setKnowledge(v ?? '')}
        />
      </div>
    </div>
  );
}
