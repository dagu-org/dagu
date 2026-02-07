import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { AlertCircle, ArrowLeft, Plus } from 'lucide-react';
import { useContext, useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';

const NAME_PATTERN = /^[a-z0-9][a-z0-9-]*[a-z0-9]$/;
const MAX_NAME_LENGTH = 63;

function validateName(name: string): string | null {
  if (!name) {
    return 'Name is required';
  }
  if (name.length === 1) {
    if (!/^[a-z0-9]$/.test(name)) {
      return 'Name must contain only lowercase letters, numbers, and hyphens';
    }
    return null;
  }
  if (name.length > MAX_NAME_LENGTH) {
    return `Name must be at most ${MAX_NAME_LENGTH} characters`;
  }
  if (!NAME_PATTERN.test(name)) {
    return 'Name must start and end with a lowercase letter or number, and contain only lowercase letters, numbers, and hyphens';
  }
  return null;
}

export default function NamespaceCreatePage() {
  const client = useClient();
  const navigate = useNavigate();
  const appBarContext = useContext(AppBarContext);

  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [defaultQueue, setDefaultQueue] = useState('');
  const [defaultWorkingDir, setDefaultWorkingDir] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [nameError, setNameError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    appBarContext.setTitle('Create Namespace');
  }, [appBarContext]);

  const handleNameChange = (value: string) => {
    setName(value);
    setNameError(value ? validateName(value) : null);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const nameValidationError = validateName(name);
    if (nameValidationError) {
      setNameError(nameValidationError);
      return;
    }

    setIsSubmitting(true);

    try {
      const body: {
        name: string;
        description?: string;
        defaults?: { queue?: string; workingDir?: string };
      } = { name };

      if (description) {
        body.description = description;
      }

      if (defaultQueue || defaultWorkingDir) {
        body.defaults = {};
        if (defaultQueue) {
          body.defaults.queue = defaultQueue;
        }
        if (defaultWorkingDir) {
          body.defaults.workingDir = defaultWorkingDir;
        }
      }

      const { error: apiError } = await client.POST('/namespaces', {
        body,
      });

      if (apiError) {
        const msg =
          (apiError as { message?: string }).message ||
          'Failed to create namespace';
        throw new Error(msg);
      }

      navigate('/namespaces');
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to create namespace'
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="flex flex-col gap-4 max-w-2xl">
      <div>
        <Button
          variant="ghost"
          size="sm"
          className="mb-2 -ml-2 text-muted-foreground"
          onClick={() => navigate('/namespaces')}
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Namespaces
        </Button>
        <h1 className="text-lg font-semibold">Create Namespace</h1>
        <p className="text-sm text-muted-foreground">
          Create a new isolation boundary for DAGs
        </p>
      </div>

      {error && (
        <div className="flex items-center gap-2 p-3 text-sm text-destructive bg-destructive/10 rounded-md">
          <AlertCircle className="h-4 w-4 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <form onSubmit={handleSubmit} className="card-obsidian p-6 space-y-5">
        <div className="space-y-1.5">
          <Label htmlFor="name" className="text-sm">
            Name <span className="text-destructive">*</span>
          </Label>
          <Input
            id="name"
            type="text"
            value={name}
            onChange={(e) => handleNameChange(e.target.value)}
            placeholder="e.g. team-alpha"
            required
            autoComplete="off"
            autoFocus
            className="h-9"
          />
          {nameError ? (
            <p className="text-xs text-destructive">{nameError}</p>
          ) : (
            <p className="text-xs text-muted-foreground">
              Lowercase letters, numbers, and hyphens. Max 63 characters.
            </p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="description" className="text-sm">
            Description
          </Label>
          <Input
            id="description"
            type="text"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional description for this namespace"
            autoComplete="off"
            className="h-9"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="defaultQueue" className="text-sm">
            Default Queue
          </Label>
          <Input
            id="defaultQueue"
            type="text"
            value={defaultQueue}
            onChange={(e) => setDefaultQueue(e.target.value)}
            placeholder="Optional default queue for DAGs in this namespace"
            autoComplete="off"
            className="h-9"
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="defaultWorkingDir" className="text-sm">
            Default Working Directory
          </Label>
          <Input
            id="defaultWorkingDir"
            type="text"
            value={defaultWorkingDir}
            onChange={(e) => setDefaultWorkingDir(e.target.value)}
            placeholder="Optional default working directory"
            autoComplete="off"
            className="h-9"
          />
        </div>

        <div className="flex justify-end gap-2 pt-2">
          <Button
            type="button"
            variant="ghost"
            onClick={() => navigate('/namespaces')}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={isSubmitting || !!nameError}>
            <Plus className="h-4 w-4" />
            {isSubmitting ? 'Creating...' : 'Create Namespace'}
          </Button>
        </div>
      </form>
    </div>
  );
}
