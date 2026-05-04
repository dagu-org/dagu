import { render, screen } from '@testing-library/react';
import React from 'react';
import { describe, expect, it } from 'vitest';
import { Button } from '../button';
import { DateRangePicker } from '../date-range-picker';
import { Select, SelectTrigger, SelectValue } from '../select';

describe('shared control surface', () => {
  it('uses a consistent default surface for toolbar controls', () => {
    render(
      <div>
        <Button variant="outline">Today</Button>
        <Select>
          <SelectTrigger aria-label="Status">
            <SelectValue placeholder="Status" />
          </SelectTrigger>
        </Select>
        <DateRangePicker
          data-testid="date-range"
          fromDate="2026-05-04T00:00"
          toDate="2026-05-04T23:59"
          onFromDateChange={() => undefined}
          onToDateChange={() => undefined}
        />
      </div>
    );

    const buttonClassName = screen.getByRole('button', {
      name: 'Today',
    }).className;
    const selectClassName = screen.getByRole('combobox', {
      name: 'Status',
    }).className;
    const dateRangeClassName = screen.getByTestId('date-range').className;

    expect(buttonClassName).toContain('h-9');
    expect(buttonClassName).toContain('bg-card');
    expect(buttonClassName).toContain('shadow-sm');

    expect(selectClassName).toContain('h-9');
    expect(selectClassName).toContain('bg-card');
    expect(selectClassName).toContain('shadow-sm');

    expect(dateRangeClassName).toContain('bg-card');
    expect(dateRangeClassName).toContain('shadow-sm');
  });
});
