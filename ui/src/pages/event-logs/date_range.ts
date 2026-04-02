import dayjs from '@/lib/dayjs';
import {
  DEFAULT_DATE_PRESET,
  type EventLogFilters,
  type SpecificPeriod,
} from './types';

export function getPresetDates(
  preset: string,
  tzOffsetInSec?: number
): { from: string; to?: string } {
  const now = dayjs();
  const startOfDay =
    tzOffsetInSec !== undefined
      ? now.utcOffset(tzOffsetInSec / 60).startOf('day')
      : now.startOf('day');

  switch (preset) {
    case 'today':
      return { from: startOfDay.format('YYYY-MM-DDTHH:mm:ss') };
    case 'yesterday':
      return {
        from: startOfDay.subtract(1, 'day').format('YYYY-MM-DDTHH:mm:ss'),
        to: startOfDay.format('YYYY-MM-DDTHH:mm:ss'),
      };
    case 'last30days':
      return {
        from: startOfDay.subtract(30, 'day').format('YYYY-MM-DDTHH:mm:ss'),
      };
    case 'thisWeek':
      return {
        from: startOfDay.startOf('week').format('YYYY-MM-DDTHH:mm:ss'),
      };
    case 'thisMonth':
      return {
        from: startOfDay.startOf('month').format('YYYY-MM-DDTHH:mm:ss'),
      };
    case 'last7days':
    default:
      return {
        from: startOfDay.subtract(7, 'day').format('YYYY-MM-DDTHH:mm:ss'),
      };
  }
}

export function getSpecificPeriodDates(
  period: SpecificPeriod,
  value: string,
  tzOffsetInSec?: number
): { from: string; to?: string } {
  const parsedDate = dayjs(value);
  if (!parsedDate.isValid()) {
    const fallback =
      tzOffsetInSec !== undefined
        ? dayjs().utcOffset(tzOffsetInSec / 60)
        : dayjs();
    return { from: fallback.startOf('day').format('YYYY-MM-DDTHH:mm:ss') };
  }

  const date =
    tzOffsetInSec !== undefined
      ? parsedDate.utcOffset(tzOffsetInSec / 60)
      : parsedDate;
  const unit = period === 'date' ? 'day' : period;
  return {
    from: date.startOf(unit).format('YYYY-MM-DDTHH:mm:ss'),
    to: date.endOf(unit).format('YYYY-MM-DDTHH:mm:ss'),
  };
}

export function parseDateFromUrl(
  value: string | null,
  tzOffsetInSec?: number
): string | undefined {
  if (!value) {
    return undefined;
  }

  if (/^\d+$/.test(value)) {
    const timestamp = Number(value);
    if (!Number.isNaN(timestamp)) {
      const parsed =
        tzOffsetInSec !== undefined
          ? dayjs.unix(timestamp).utcOffset(tzOffsetInSec / 60)
          : dayjs.unix(timestamp);
      return parsed.format('YYYY-MM-DDTHH:mm:ss');
    }
  }

  if (value.includes('T') && value.length >= 16) {
    return value;
  }

  return undefined;
}

export function formatDateForApi(
  dateString: string | undefined,
  tzOffsetInSec?: number
): string | undefined {
  if (!dateString) {
    return undefined;
  }
  const dateWithSeconds =
    dateString.split(':').length < 3 ? `${dateString}:00` : dateString;
  if (tzOffsetInSec !== undefined) {
    return dayjs(dateWithSeconds)
      .utcOffset(tzOffsetInSec / 60, true)
      .toISOString();
  }
  return dayjs(dateWithSeconds).toISOString();
}

export function formatTimestamp(
  timestamp: string,
  tzOffsetInSec?: number
): string {
  const parsed =
    tzOffsetInSec !== undefined
      ? dayjs(timestamp).utcOffset(tzOffsetInSec / 60)
      : dayjs(timestamp);
  return parsed.format('YYYY-MM-DD HH:mm:ss');
}

export function formatTimezoneOffset(tzOffsetInSec?: number): string {
  if (tzOffsetInSec === undefined) {
    return '';
  }
  const offsetInMinutes = tzOffsetInSec / 60;
  const hours = Math.floor(Math.abs(offsetInMinutes) / 60);
  const minutes = Math.abs(offsetInMinutes) % 60;
  const sign = offsetInMinutes >= 0 ? '+' : '-';
  return `(${sign}${hours.toString().padStart(2, '0')}:${minutes
    .toString()
    .padStart(2, '0')})`;
}

export function createDefaultEventLogFilters(
  tzOffsetInSec?: number
): EventLogFilters {
  const dates = getPresetDates(DEFAULT_DATE_PRESET, tzOffsetInSec);
  return {
    kind: 'all',
    type: 'all',
    dagName: '',
    automataName: '',
    dagRunId: '',
    attemptId: '',
    fromDate: dates.from,
    toDate: dates.to,
    dateRangeMode: 'preset',
    datePreset: DEFAULT_DATE_PRESET,
    specificPeriod: 'date',
    specificValue: dayjs().format('YYYY-MM-DD'),
  };
}
