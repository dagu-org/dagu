import { useState, useEffect, useContext } from 'react';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useConfig } from '@/contexts/ConfigContext';
import { components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import { getAuthHeaders } from '@/lib/authHeaders';

type SkillResponse = components['schemas']['SkillResponse'];

function generateSlugId(name: string): string {
  return name.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
}

interface SkillFormModalProps {
  open: boolean;
  skill?: SkillResponse;
  onClose: () => void;
  onSuccess: () => void;
}

export function SkillFormModal({ open, skill, onClose, onSuccess }: SkillFormModalProps) {
  const config = useConfig();
  const appBarContext = useContext(AppBarContext);
  const isEditing = !!skill;

  const [skillId, setSkillId] = useState('');
  const [customId, setCustomId] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [knowledge, setKnowledge] = useState('');
  const [version, setVersion] = useState('');
  const [author, setAuthor] = useState('');
  const [tagsInput, setTagsInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (open && skill) {
      setSkillId(skill.id);
      setName(skill.name);
      setDescription(skill.description ?? '');
      setKnowledge(skill.knowledge);
      setVersion(skill.version ?? '');
      setAuthor(skill.author ?? '');
      setTagsInput(skill.tags?.join(', ') ?? '');
    } else if (open && !skill) {
      resetForm();
    }
    if (open) {
      setError(null);
    }
  }, [open, skill]);

  function resetForm() {
    setSkillId('');
    setCustomId(false);
    setName('');
    setDescription('');
    setKnowledge('');
    setVersion('');
    setAuthor('');
    setTagsInput('');
  }

  function parseTags(input: string): string[] {
    return input
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError(null);

    try {
      const remoteNode = encodeURIComponent(appBarContext.selectedRemoteNode || 'local');
      const tags = parseTags(tagsInput);

      const body: Record<string, unknown> = {
        name,
        knowledge,
        description: description || undefined,
        version: version || undefined,
        author: author || undefined,
        tags: tags.length > 0 ? tags : undefined,
      };

      if (!isEditing && skillId) {
        body.id = skillId;
      }

      const url = isEditing
        ? `${config.apiURL}/settings/agent/skills/${skill.id}?remoteNode=${remoteNode}`
        : `${config.apiURL}/settings/agent/skills?remoteNode=${remoteNode}`;

      const response = await fetch(url, {
        method: isEditing ? 'PATCH' : 'POST',
        headers: getAuthHeaders(),
        body: JSON.stringify(body),
      });

      if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || `Failed to ${isEditing ? 'update' : 'create'} skill`);
      }

      resetForm();
      onSuccess();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'An error occurred');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-lg max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{isEditing ? 'Edit Skill' : 'Create Skill'}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit}>
          <div className="space-y-3 py-3">
            {error && (
              <div className="p-3 text-sm text-destructive bg-destructive/10 rounded-md">
                {error}
              </div>
            )}

            <div className="space-y-1.5">
              <Label htmlFor="skill-name" className="text-sm">Name</Label>
              <Input
                id="skill-name"
                value={name}
                onChange={(e) => {
                  const v = e.target.value;
                  setName(v);
                  if (!isEditing && !customId) {
                    setSkillId(generateSlugId(v));
                  }
                }}
                placeholder="My Skill"
                className="h-8"
                required
              />
            </div>

            {!isEditing && (
              <div className="space-y-1.5">
                <Label htmlFor="skill-id" className="text-sm">ID (optional)</Label>
                <Input
                  id="skill-id"
                  value={skillId}
                  onChange={(e) => {
                    setSkillId(e.target.value);
                    setCustomId(true);
                  }}
                  placeholder="Auto-generated from name if empty"
                  className="h-8"
                />
                <p className="text-xs text-muted-foreground">
                  Lowercase letters, numbers, and hyphens only
                </p>
              </div>
            )}

            <div className="space-y-1.5">
              <Label htmlFor="skill-description" className="text-sm">Description (optional)</Label>
              <Input
                id="skill-description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="What this skill does"
                className="h-8"
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="skill-knowledge" className="text-sm">Knowledge</Label>
              <textarea
                id="skill-knowledge"
                value={knowledge}
                onChange={(e) => setKnowledge(e.target.value)}
                placeholder="Domain knowledge and instructions for the AI agent..."
                className="w-full h-48 p-3 text-sm font-mono bg-muted/50 border rounded-md resize-y focus:outline-none focus:ring-1 focus:ring-ring"
                required
              />
              <p className="text-xs text-muted-foreground">
                Markdown-formatted instructions the agent will follow when this skill is enabled
              </p>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label htmlFor="skill-version" className="text-sm">Version (optional)</Label>
                <Input
                  id="skill-version"
                  value={version}
                  onChange={(e) => setVersion(e.target.value)}
                  placeholder="1.0.0"
                  className="h-8"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="skill-author" className="text-sm">Author (optional)</Label>
                <Input
                  id="skill-author"
                  value={author}
                  onChange={(e) => setAuthor(e.target.value)}
                  placeholder="Author name"
                  className="h-8"
                />
              </div>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="skill-tags" className="text-sm">Tags (optional)</Label>
              <Input
                id="skill-tags"
                value={tagsInput}
                onChange={(e) => setTagsInput(e.target.value)}
                placeholder="kubernetes, devops, monitoring"
                className="h-8"
              />
              <p className="text-xs text-muted-foreground">
                Comma-separated tags for filtering
              </p>
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} size="sm" className="h-8">
              Cancel
            </Button>
            <Button type="submit" disabled={isLoading || !name || !knowledge} size="sm" className="h-8">
              {isLoading ? 'Saving...' : isEditing ? 'Save Changes' : 'Create Skill'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
