import React from 'react';

import { cn } from '@/lib/utils';

type AutomataAvatarProps = {
  name: string;
  nickname?: string | null;
  iconUrl?: string | null;
  className?: string;
  iconClassName?: string;
};

export function AutomataAvatar({
  name,
  nickname,
  iconUrl,
  className,
  iconClassName,
}: AutomataAvatarProps): React.ReactElement {
  const src = iconUrl?.trim() || '';
  const [imageFailed, setImageFailed] = React.useState(false);

  React.useEffect(() => {
    setImageFailed(false);
  }, [src]);

  const label = nickname?.trim() || name;
  const initials = label
    .split(/[\s_-]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() || '')
    .join('') || label.slice(0, 2).toUpperCase();

  return (
    <div
      className={cn(
        'flex shrink-0 items-center justify-center overflow-hidden rounded-xl border bg-muted/40 text-muted-foreground shadow-sm',
        className
      )}
      title={label}
    >
      {src && !imageFailed ? (
        <img
          src={src}
          alt=""
          loading="lazy"
          referrerPolicy="no-referrer"
          className="h-full w-full object-cover"
          onError={() => setImageFailed(true)}
        />
      ) : (
        <div
          className={cn(
            'relative flex h-full w-full items-center justify-center overflow-hidden bg-linear-to-br from-slate-100 via-white to-slate-200 text-slate-700 dark:from-slate-900 dark:via-slate-950 dark:to-slate-800 dark:text-slate-100',
            iconClassName
          )}
          aria-hidden="true"
        >
          <div className="absolute inset-x-[14%] top-[14%] h-[30%] rounded-full bg-white/75 ring-1 ring-slate-300/60 dark:bg-slate-800/90 dark:ring-slate-700/80" />
          <div className="absolute inset-x-[10%] bottom-[10%] h-[32%] rounded-[999px] bg-slate-900/10 ring-1 ring-slate-400/30 dark:bg-white/10 dark:ring-white/10" />
          <div className="absolute inset-x-[18%] bottom-[16%] h-[18%] rounded-full bg-slate-900/8 dark:bg-white/8" />
          <span className="relative z-10 select-none text-sm font-semibold tracking-[0.18em] text-slate-700 uppercase dark:text-slate-100">
            {initials}
          </span>
        </div>
      )}
    </div>
  );
}
