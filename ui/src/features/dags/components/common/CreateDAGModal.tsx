import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Plus } from 'lucide-react';
import { useContext, useState } from 'react';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useConfig } from '../../../../contexts/ConfigContext';
import { useClient } from '../../../../hooks/api';

/**
 * CreateDAGModal displays a button that opens a modal to create a new DAG
 * and redirects to the DAG specification page after creation
 */
function CreateDAGModal() {
  const appBarContext = useContext(AppBarContext);
  const client = useClient();
  const config = useConfig();
  const [name, setName] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);

  if (!config.permissions.writeDags) {
    return null;
  }

  const handleNameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setName(e.target.value);
    // Clear error when user types
    if (error) setError(null);
  };

  // Regex pattern for valid DAG names
  const DAG_NAME_PATTERN = /^[a-zA-Z0-9_.-]+$/;

  const validateName = (): boolean => {
    if (!name.trim()) {
      setError('DAG name cannot be empty');
      return false;
    }
    if (!DAG_NAME_PATTERN.test(name)) {
      setError(
        'DAG name can only contain letters, numbers, underscores, dots, and hyphens'
      );
      return false;
    }
    return true;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!validateName()) return;

    setIsLoading(true);

    try {
      const { error } = await client.POST('/dags', {
        params: {
          query: {
            remoteNode: appBarContext.selectedRemoteNode || 'local',
          },
        },
        body: {
          name,
        },
      });

      if (error) {
        setError(error.message || 'An error occurred');
        setIsLoading(false);
        return;
      }

      // Success - close modal and redirect
      setIsOpen(false);

      // Redirect to the DAG specification page
      const basePath = window.location.pathname.split('/dags')[0] || '';
      window.location.href = `${basePath}/dags/${name}/spec`;
    } catch {
      setError('An unexpected error occurred');
      setIsLoading(false);
    }
  };

  return (
    <Dialog open={isOpen} onOpenChange={setIsOpen}>
      <DialogTrigger asChild>
        <Button
          aria-label="Create new DAG"
          className="flex items-center gap-1.5 bg-primary text-white font-medium px-3 py-1 text-sm rounded-md shadow-sm hover:bg-primary/90 focus:outline-none focus:ring-1 focus:ring-primary focus:ring-offset-1 transition cursor-pointer h-8"
        >
          <Plus className="w-3.5 h-3.5" aria-hidden="true" />
          <span>New</span>
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[425px]">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create New DAG</DialogTitle>
            <DialogDescription>
              Enter a name for your new DAG. Only letters, numbers, underscores,
              dots, and hyphens are allowed.
              <div className="mt-1 font-mono text-xs bg-slate-100 p-1 rounded">
                Pattern: ^[a-zA-Z0-9_.-]+$
              </div>
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-4 items-center gap-4">
              <Label htmlFor="name" className="text-right">
                DAG Name
              </Label>
              <Input
                id="name"
                value={name}
                onChange={handleNameChange}
                className="col-span-3"
                placeholder="my_new_dag"
                pattern="^[a-zA-Z0-9_.-]+$"
                autoFocus
              />
            </div>
            {error && (
              <div className="text-destructive text-sm px-4">{error}</div>
            )}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setIsOpen(false)}
              disabled={isLoading}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isLoading}>
              {isLoading ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export default CreateDAGModal;
