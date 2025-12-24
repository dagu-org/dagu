import { Table as RadixTable } from '@radix-ui/themes';
import * as React from 'react';

import { cn } from '@/lib/utils';

function Table({
  className,
  children,
  ...props
}: React.ComponentProps<typeof RadixTable.Root>) {
  return (
    <div className="w-full overflow-hidden border border-border/50">
      <RadixTable.Root
        data-slot="table"
        variant="ghost"
        className={cn('w-full', className)}
        {...props}
      >
        {children}
      </RadixTable.Root>
    </div>
  );
}

function TableHeader({
  className,
  ...props
}: React.ComponentProps<typeof RadixTable.Header>) {
  return (
    <RadixTable.Header
      data-slot="table-header"
      className={cn(className)}
      {...props}
    />
  );
}

function TableBody({
  className,
  ...props
}: React.ComponentProps<typeof RadixTable.Body>) {
  return (
    <RadixTable.Body
      data-slot="table-body"
      className={cn(className)}
      {...props}
    />
  );
}

function TableRow({
  className,
  ...props
}: React.ComponentProps<typeof RadixTable.Row>) {
  return (
    <RadixTable.Row
      data-slot="table-row"
      className={cn(
        'hover:bg-muted/50 transition-colors bg-surface',
        className
      )}
      {...props}
    />
  );
}

function TableHead({
  className,
  ...props
}: React.ComponentProps<typeof RadixTable.ColumnHeaderCell>) {
  return (
    <RadixTable.ColumnHeaderCell
      data-slot="table-head"
      className={cn('text-foreground font-medium', className)}
      {...props}
    />
  );
}

function TableCell({
  className,
  ...props
}: React.ComponentProps<typeof RadixTable.Cell>) {
  return (
    <RadixTable.Cell
      data-slot="table-cell"
      className={cn(className)}
      {...props}
    />
  );
}

function TableCaption({
  className,
  ...props
}: React.ComponentProps<'caption'>) {
  return (
    <caption
      data-slot="table-caption"
      className={cn('text-muted-foreground mt-4 text-sm', className)}
      {...props}
    />
  );
}

function TableFooter({
  className,
  children,
  ...props
}: React.ComponentProps<'tfoot'>) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(
        'bg-muted/50 border-t font-medium [&>tr]:last:border-b-0',
        className
      )}
      {...props}
    >
      {children}
    </tfoot>
  );
}

export {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
};
