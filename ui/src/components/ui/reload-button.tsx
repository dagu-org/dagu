import React, { useState } from 'react';

interface ReloadButtonProps {
  onReload: () => void | Promise<void>;
  isLoading?: boolean;
  disabled?: boolean;
  className?: string;
  title?: string;
}

export const ReloadButton: React.FC<ReloadButtonProps> = ({
  onReload,
  isLoading = false,
  disabled = false,
  className = '',
  title = 'Reload',
}) => {
  const [isReloading, setIsReloading] = useState(false);

  const handleClick = async () => {
    setIsReloading(true);
    try {
      await onReload();
      // Brief success state
      setTimeout(() => setIsReloading(false), 500);
    } catch {
      setIsReloading(false);
    }
  };

  const isDisabled = disabled || isLoading || isReloading;
  
  return (
    <button
      onClick={handleClick}
      disabled={isDisabled}
      className={`
        inline-flex items-center justify-center px-2.5 py-1 rounded-full text-xs
        transition-all duration-200 ease-in-out transform
        ${
          isReloading
            ? 'bg-blue-500 text-white scale-90'
            : isDisabled
            ? 'bg-muted text-muted-foreground cursor-not-allowed'
            : 'bg-accent text-muted-foreground hover:bg-accent hover:scale-110 active:scale-95'
        }
        ${className}
      `}
      title={title}
    >
      <svg
        className={`w-4 h-4 ${isReloading || isLoading ? 'animate-spin' : ''} transition-transform duration-200`}
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
        />
      </svg>
    </button>
  );
};