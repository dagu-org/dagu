import { Button } from '@/components/ui/button';
import { ExternalLink, Maximize2, Minimize2, X } from 'lucide-react';
import React, { useEffect, useState } from 'react';

type LogSideModalProps = {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
  isInModal?: boolean;
  dagName?: string;
  workflowId?: string;
  stepName?: string;
  logType?: 'execution' | 'step';
};

/**
 * LogSideModal displays log content in a side panel that appears from the right
 * This creates a developer-friendly experience for viewing logs without hiding the DAG details
 */
const LogSideModal: React.FC<LogSideModalProps> = ({
  isOpen,
  onClose,
  title,
  children,
  isInModal = false,
  dagName = '',
  workflowId = '',
  stepName = '',
  logType = 'execution',
}) => {
  // State to track whether the modal is expanded
  const [isExpanded, setIsExpanded] = useState(false);
  
  // State to track if we're on mobile
  const [isMobile, setIsMobile] = useState(false);

  // Check if screen is mobile size
  useEffect(() => {
    const checkIsMobile = () => {
      setIsMobile(window.innerWidth < 768); // md breakpoint
    };

    checkIsMobile();
    window.addEventListener('resize', checkIsMobile);

    return () => {
      window.removeEventListener('resize', checkIsMobile);
    };
  }, []);

  // Add keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      // Close modal with Escape key
      if (event.key === 'Escape') {
        onClose();
      }
    };

    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown);
    }

    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  // Calculate z-index and positioning based on whether it's in a modal or not
  const zIndex = isInModal ? 60 : 50; // Higher z-index when in modal

  // Determine width and positioning based on mobile/expanded state
  const width = isMobile || isExpanded ? 'w-full' : isInModal ? 'w-1/2' : 'w-2/5';
  const positioning = isMobile ? 'inset-0' : 'top-0 bottom-0 right-0';

  // Handle clicks outside the modal
  const handleOutsideClick = (e: React.MouseEvent) => {
    // Stop propagation to prevent closing the parent modal
    e.stopPropagation();
    onClose();
  };

  // Toggle expanded state
  const toggleExpand = () => {
    setIsExpanded(!isExpanded);
  };

  // Generate URL for opening log in new tab
  const getLogUrl = () => {
    const baseUrl = `/dags/${dagName}`;
    const searchParams = new URLSearchParams();

    if (workflowId) {
      searchParams.set('workflowId', workflowId);
    }

    if (logType === 'step' && stepName) {
      searchParams.set('step', stepName);
      return `${baseUrl}/log?${searchParams.toString()}`;
    } else {
      return `${baseUrl}/workflow-log?${searchParams.toString()}`;
    }
  };

  // Open log in new tab
  const openInNewTab = () => {
    window.open(getLogUrl(), '_blank');
  };

  return (
    <>
      {/* Invisible backdrop for capturing clicks outside the modal - always present */}
      <div
        className={`fixed inset-0 h-screen w-screen ${!isInModal || isMobile ? 'bg-black/50' : 'bg-transparent'}`}
        style={{ zIndex: zIndex }}
        onClick={isMobile ? onClose : handleOutsideClick}
      />

      {/* Modal - full screen on mobile, side panel on desktop */}
      <div
        className={`fixed ${positioning} ${width} h-screen bg-background ${
          isMobile 
            ? 'border border-border rounded-none' 
            : 'border-l border-border'
        } shadow-xl overflow-hidden flex flex-col ${
          isMobile ? 'animate-in fade-in-0 slide-in-from-bottom-10' : 'slide-in-from-right'
        }`}
        style={{ zIndex: zIndex + 1 }} // Make sure modal is above backdrop
        onClick={(e) => e.stopPropagation()} // Prevent clicks inside the modal from closing it
      >
        <div className={`flex justify-between items-center ${isMobile ? 'p-3' : 'p-4'} border-b`}>
          <h2 className={`${isMobile ? 'text-base' : 'text-lg'} font-semibold`}>{title}</h2>
          <div className="flex gap-2">
            {/* Hide expand/minimize button on mobile since it's always full screen */}
            {!isMobile && (
              <Button
                variant="outline"
                size="icon"
                onClick={toggleExpand}
                title={isExpanded ? 'Minimize' : 'Expand'}
                className="relative group"
              >
                {isExpanded ? (
                  <Minimize2 className="h-4 w-4" />
                ) : (
                  <Maximize2 className="h-4 w-4" />
                )}
              </Button>
            )}
            <Button
              variant="outline"
              size="icon"
              onClick={openInNewTab}
              title="Open in new tab"
              className="relative group"
            >
              <ExternalLink className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              size="icon"
              onClick={onClose}
              title="Close (Esc)"
              className="relative group"
            >
              <X className="h-4 w-4" />
              {!isMobile && (
                <span className="absolute -bottom-1 -right-1 bg-primary text-primary-foreground text-[10px] font-medium px-1 rounded-sm opacity-0 group-hover:opacity-100 transition-opacity">
                  Esc
                </span>
              )}
            </Button>
          </div>
        </div>
        <div className={`flex-1 overflow-auto ${isMobile ? 'p-2' : 'p-4'}`}>{children}</div>
      </div>

      {/* Animation is handled via CSS classes in global.css */}
    </>
  );
};

export default LogSideModal;
