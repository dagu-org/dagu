import { Check } from 'lucide-react';
import React, { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';

interface SimpleToastProps {
  message: string;
  duration?: number;
  onClose?: () => void;
}

// Minimum duration to ensure animations complete properly
const MIN_DURATION = 500;

export const SimpleToast: React.FC<SimpleToastProps> = ({
  message,
  duration = 1800,
  onClose,
}) => {
  const [isVisible, setIsVisible] = useState(true);
  const [animationState, setAnimationState] = useState<
    'entering' | 'visible' | 'exiting'
  >('entering');
  const [checkAnimated, setCheckAnimated] = useState(false);

  // Ensure duration is at least MIN_DURATION
  const safeDuration = Math.max(duration, MIN_DURATION);

  useEffect(() => {
    // Enter animation
    const enterTimer = setTimeout(() => {
      setAnimationState('visible');
    }, 20);

    // Checkmark draw animation
    const checkTimer = setTimeout(() => {
      setCheckAnimated(true);
    }, 150);

    // Start exit animation (at least 350ms before end)
    const exitTimer = setTimeout(() => {
      setAnimationState('exiting');
    }, safeDuration - 350);

    // Remove from DOM
    const removeTimer = setTimeout(() => {
      setIsVisible(false);
      if (onClose) onClose();
    }, safeDuration);

    return () => {
      clearTimeout(enterTimer);
      clearTimeout(checkTimer);
      clearTimeout(exitTimer);
      clearTimeout(removeTimer);
    };
  }, [safeDuration, onClose]);

  if (!isVisible) return null;

  const getAnimationClasses = () => {
    switch (animationState) {
      case 'entering':
        return 'opacity-0 scale-90';
      case 'visible':
        return 'opacity-100 scale-100';
      case 'exiting':
        return 'opacity-0 scale-95';
    }
  };

  return createPortal(
    <div className="fixed inset-0 z-[100] flex items-center justify-center pointer-events-none">
      <div
        className={`
          pointer-events-auto
          flex flex-col items-center justify-center gap-3
          w-32 h-32
          bg-popover/80
          backdrop-blur-xl
          rounded-[20px]
          border border-border/50
          shadow-toast
          transition-all duration-300 ease-out
          ${getAnimationClasses()}
        `}
      >
        {/* Animated checkmark circle */}
        <div className="relative w-12 h-12">
          {/* Circle background */}
          <div
            className={`
              absolute inset-0 rounded-full
              border-[2.5px] border-success
              transition-all duration-300 ease-out
              ${checkAnimated ? 'opacity-100 scale-100' : 'opacity-0 scale-75'}
            `}
          />
          {/* Checkmark icon */}
          <div
            className={`
              absolute inset-0 flex items-center justify-center
              transition-all duration-300 ease-out delay-100
              ${checkAnimated ? 'opacity-100 scale-100' : 'opacity-0 scale-50'}
            `}
          >
            <Check className="h-7 w-7 text-success" strokeWidth={3} />
          </div>
        </div>

        {/* Message */}
        <span
          className={`
            text-sm font-medium text-foreground/90 text-center px-2
            transition-all duration-200 delay-150
            ${checkAnimated ? 'opacity-100' : 'opacity-0'}
          `}
        >
          {message}
        </span>
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
