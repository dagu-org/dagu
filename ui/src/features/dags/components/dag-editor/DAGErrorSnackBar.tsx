/**
 * DAGErrorSnackBar component displays a snackbar with error messages.
 *
 * @module features/dags/components/dag-editor
 */
import { Alert } from '@/components/ui/alert';
import {
  Toast,
  ToastClose,
  ToastDescription,
  ToastProvider,
  ToastTitle,
} from '@/components/ui/toast';

/**
 * Props for the DAGErrorSnackBar component
 */
type DAGErrorSnackBarProps = {
  /** Whether the snackbar is open */
  open: boolean;
  /** Function to set the open state */
  setOpen: (open: boolean) => void;
  /** List of error messages */
  errors: string[];
};

/**
 * DAGErrorSnackBar displays a snackbar with error messages
 * that automatically hides after a timeout
 */
const DAGErrorSnackBar = ({ open, setOpen, errors }: DAGErrorSnackBarProps) => {
  /**
   * Handle closing the snackbar
   */
  const handleClose = () => {
    setOpen(false);
  };

  // If not open, don't render anything
  if (!open) return null;

  return (
    <ToastProvider>
      <div className="fixed top-4 left-1/2 transform -translate-x-1/2 z-50 w-[20vw] max-w-md">
        <Toast variant="destructive" className="bg-white border-red-500">
          <div className="flex flex-col items-center w-full">
            <ToastTitle className="text-red-500 text-xl font-bold">
              Error Detected
            </ToastTitle>

            <ToastDescription className="text-red-400 text-lg mt-1">
              Please check the following errors:
            </ToastDescription>

            <div className="w-full mt-2">
              {errors.map((error, index) => (
                <Alert
                  key={index}
                  variant="destructive"
                  className="mb-2 bg-red-50 text-red-400 text-sm py-2"
                >
                  {error}
                </Alert>
              ))}
            </div>
          </div>

          <ToastClose onClick={handleClose} />
        </Toast>
      </div>
    </ToastProvider>
  );
};

export default DAGErrorSnackBar;
