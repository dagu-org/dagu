import React from 'react';
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from '@/components/ui/pagination'; // Import shadcn Pagination components
import { Input } from '@/components/ui/input'; // Import shadcn Input
import { Label } from '@/components/ui/label'; // Import shadcn Label

/**
 * Props for the DAGPagination component
 */
type DAGPaginationProps = {
  /** Total number of pages */
  totalPages: number;
  /** Current page number */
  page: number;
  /** Number of items per page */
  pageLimit: number;
  /** Callback for page change */
  pageChange: (page: number) => void;
  /** Callback for page limit change */
  onPageLimitChange: (pageLimit: number) => void;
};

/**
 * Helper function to generate pagination items with ellipsis
 */
const generatePaginationItems = (
  currentPage: number,
  totalPages: number,
  onPageChange: (page: number) => void
) => {
  const items = [];
  const maxPagesToShow = 5; // Adjust number of page links shown
  const halfMaxPages = Math.floor(maxPagesToShow / 2);

  // Always show Previous button
  items.push(
    <PaginationItem key="prev">
      <PaginationPrevious
        href="#" // Use href="#" for non-navigation links or handle appropriately
        onClick={(e) => {
          e.preventDefault();
          if (currentPage > 1) onPageChange(currentPage - 1);
        }}
        aria-disabled={currentPage <= 1}
        className={currentPage <= 1 ? 'pointer-events-none opacity-50' : ''}
      />
    </PaginationItem>
  );

  // Logic to show page numbers and ellipsis
  if (totalPages <= maxPagesToShow + 2) {
    // Show all pages if not too many
    for (let i = 1; i <= totalPages; i++) {
      items.push(
        <PaginationItem key={i}>
          <PaginationLink
            href="#"
            onClick={(e) => {
              e.preventDefault();
              onPageChange(i);
            }}
            isActive={i === currentPage}
          >
            {i}
          </PaginationLink>
        </PaginationItem>
      );
    }
  } else {
    // Show first page
    items.push(
      <PaginationItem key={1}>
        <PaginationLink
          href="#"
          onClick={(e) => {
            e.preventDefault();
            onPageChange(1);
          }}
          isActive={1 === currentPage}
        >
          1
        </PaginationLink>
      </PaginationItem>
    );

    // Ellipsis after first page?
    if (currentPage > halfMaxPages + 2) {
      items.push(
        <PaginationItem key="ellipsis-start">
          <PaginationEllipsis />
        </PaginationItem>
      );
    }

    // Middle pages
    const startPage = Math.max(2, currentPage - halfMaxPages);
    const endPage = Math.min(totalPages - 1, currentPage + halfMaxPages);

    for (let i = startPage; i <= endPage; i++) {
      items.push(
        <PaginationItem key={i}>
          <PaginationLink
            href="#"
            onClick={(e) => {
              e.preventDefault();
              onPageChange(i);
            }}
            isActive={i === currentPage}
          >
            {i}
          </PaginationLink>
        </PaginationItem>
      );
    }

    // Ellipsis before last page?
    if (currentPage < totalPages - halfMaxPages - 1) {
      items.push(
        <PaginationItem key="ellipsis-end">
          <PaginationEllipsis />
        </PaginationItem>
      );
    }

    // Show last page
    items.push(
      <PaginationItem key={totalPages}>
        <PaginationLink
          href="#"
          onClick={(e) => {
            e.preventDefault();
            onPageChange(totalPages);
          }}
          isActive={totalPages === currentPage}
        >
          {totalPages}
        </PaginationLink>
      </PaginationItem>
    );
  }

  // Always show Next button
  items.push(
    <PaginationItem key="next">
      <PaginationNext
        href="#"
        onClick={(e) => {
          e.preventDefault();
          if (currentPage < totalPages) onPageChange(currentPage + 1);
        }}
        aria-disabled={currentPage >= totalPages}
        className={
          currentPage >= totalPages ? 'pointer-events-none opacity-50' : ''
        }
      />
    </PaginationItem>
  );

  return items;
};

/**
 * DAGPagination provides pagination controls with customizable page size
 */
const DAGPagination = ({
  totalPages,
  page,
  pageChange,
  pageLimit,
  onPageLimitChange,
}: DAGPaginationProps) => {
  // State for the input field value
  const [inputValue, setInputValue] = React.useState(pageLimit.toString());

  // Update input value when pageLimit prop changes
  React.useEffect(() => {
    setInputValue(pageLimit.toString());
  }, [pageLimit]);

  /**
   * Handle input change for page limit
   */
  const handleLimitChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const value = event.target.value;
    setInputValue(value);
  };

  /**
   * Apply the page limit change
   */
  const commitChange = () => {
    const numValue = parseInt(inputValue);
    if (!isNaN(numValue) && numValue > 0) {
      onPageLimitChange(numValue);
    } else {
      // Reset to current page limit if invalid input
      setInputValue(pageLimit.toString());
    }
  };

  /**
   * Handle Enter key press to commit changes
   */
  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter') {
      commitChange();
      event.preventDefault();
      (event.target as HTMLInputElement).blur(); // Remove focus after Enter
    }
  };

  return (
    // Replace MUI Box with div and Tailwind classes
    <div className="flex flex-row items-center justify-center gap-4 mt-2">
      {' '}
      {/* Use gap-4 for spacing */}
      {/* Items per page input */}
      <div className="flex items-center gap-2">
        <Label
          htmlFor="itemsPerPage"
          className="whitespace-nowrap text-sm text-muted-foreground"
        >
          Items per page:
        </Label>
        <Input
          id="itemsPerPage"
          type="number"
          min="1"
          className="h-8 w-[70px]" // Adjust size
          value={inputValue}
          onChange={handleLimitChange}
          onBlur={commitChange} // Commit on blur
          onKeyDown={handleKeyDown} // Commit on Enter
        />
      </div>
      {/* Pagination controls */}
      <Pagination>
        <PaginationContent>
          {generatePaginationItems(page, totalPages, pageChange)}
        </PaginationContent>
      </Pagination>
    </div>
  );
};

export default DAGPagination;
