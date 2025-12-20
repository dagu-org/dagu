import { CheckCircle } from 'lucide-react';
import React, { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';

interface SimpleToastProps {
  message: string;
  duration?: number;
  onClose?: () => void;
}

export const SimpleToast: React.FC<SimpleToastProps> = ({
  message,
  duration = 1500,
  onClose,
}) => {
  const [isVisible, setIsVisible] = useState(true);
  const [isFading, setIsFading] = useState(false);

  useEffect(() => {
    const fadeTimer = setTimeout(() => {
      setIsFading(true);
    }, duration - 300); // Start fading 300ms before removal

    const removeTimer = setTimeout(() => {
      setIsVisible(false);
      if (onClose) onClose();
    }, duration);

    return () => {
      clearTimeout(fadeTimer);
      clearTimeout(removeTimer);
    };
  }, [duration, onClose]);

  const handleClose = () => {
    setIsFading(true);
    setTimeout(() => {
      setIsVisible(false);
      if (onClose) onClose();
    }, 300);
  };

  if (!isVisible) return null;

  return createPortal(
    <div
      className={`fixed bottom-4 right-4 z-[100] max-w-sm bg-green-50 border border-green-200 text-green-800 rounded-md p-3 cursor-pointer hover:opacity-90 transition-all duration-300 ${
        isFading ? 'opacity-0 translate-y-2' : 'opacity-100 translate-y-0'
      }`}
      onClick={handleClose}
    >
      <div className="flex items-start gap-2">
        <CheckCircle className="h-4 w-4 text-green-500 flex-shrink-0 mt-0.5" />
        <div className="text-sm font-medium whitespace-pre-line break-words">{message}</div>
      </div>
    </div>,
    document.body
  );
};

interface ToastManagerProps {
  children: React.ReactNode;
}

interface ToastContextType {
  showToast: (message: string, duration?: number) => void;
}

export const ToastContext = React.createContext<ToastContextType>({
  showToast: () => {},
});

export const ToastProvider: React.FC<ToastManagerProps> = ({ children }) => {
  const [toast, setToast] = useState<{
    message: string;
    duration: number;
    id: number;
  } | null>(null);

  const showToast = (message: string, duration = 1500) => {
    setToast({ message, duration, id: Date.now() });
  };

  const handleClose = () => {
    setToast(null);
  };

  return (
    <ToastContext.Provider value={{ showToast }}>
      {children}
      {toast && (
        <SimpleToast
          key={toast.id}
          message={toast.message}
          duration={toast.duration}
          onClose={handleClose}
        />
      )}
    </ToastContext.Provider>
  );
};

export const useSimpleToast = () => {
  const context = React.useContext(ToastContext);
  if (context === undefined) {
    throw new Error('useSimpleToast must be used within a ToastProvider');
  }
  return context;
};
